package async

import (
	"context"
	"sync"
	"time"

	"log/slog"

	"github.com/joseph-ayodele/receipts-tracker/internal/core"
)

type ProcessorQueue struct {
	proc    *core.Processor
	logger  *slog.Logger
	workers int
	timeout time.Duration

	ch   chan Job
	wg   sync.WaitGroup
	once sync.Once

	mu     sync.Mutex
	closed bool
}

type Option func(*ProcessorQueue)

func WithWorkers(n int) Option {
	return func(q *ProcessorQueue) {
		if n > 0 {
			q.workers = n
		}
	}
}
func WithQueueSize(n int) Option {
	return func(q *ProcessorQueue) {
		if n > 0 {
			q.ch = make(chan Job, n)
		}
	}
}
func WithProcessTimeout(d time.Duration) Option {
	return func(q *ProcessorQueue) {
		if d > 0 {
			q.timeout = d
		}
	}
}

func NewProcessorQueue(proc *core.Processor, logger *slog.Logger, opts ...Option) *ProcessorQueue {
	q := &ProcessorQueue{
		proc:    proc,
		logger:  logger,
		workers: 4,
		timeout: 3 * time.Minute,
		ch:      make(chan Job, 256),
	}
	for _, o := range opts {
		o(q)
	}
	q.start()
	return q
}

func (q *ProcessorQueue) start() {
	q.once.Do(func() {
		for i := 0; i < q.workers; i++ {
			q.wg.Add(1)
			go func(workerID int) {
				defer q.wg.Done()
				q.logger.Info("worker started", "worker_id", workerID)

				for job := range q.ch {
					ctx, cancel := context.WithTimeout(context.Background(), q.timeout)
					_, err := q.proc.ProcessFile(ctx, job.FileID)
					cancel()

					if err != nil {
						q.logger.Error("processing failed", "worker_id", workerID, "file_id", job.FileID, "error", err)
					} else {
						q.logger.Info("processed file successfully", "worker_id", workerID, "file_id", job.FileID)
					}
				}

				q.logger.Info("worker stopped", "worker_id", workerID)
			}(i + 1)
		}
	})
}

func (q *ProcessorQueue) Enqueue(_ context.Context, job Job) error {
	q.mu.Lock()
	defer q.mu.Unlock()
	if q.closed {
		q.logger.Warn("cannot enqueue: queue is shutting down", "file_id", job.FileID)
		return nil
	}
	select {
	case q.ch <- job:
		q.logger.Info("queued file for processing", "file_id", job.FileID, "force", job.Force)
	default:
		q.logger.Warn("queue full, applying backpressure", "file_id", job.FileID)
		q.ch <- job
	}
	return nil
}

func (q *ProcessorQueue) Shutdown(ctx context.Context) {
	q.mu.Lock()
	if q.closed {
		q.mu.Unlock()
		return
	}
	q.closed = true
	close(q.ch)
	q.mu.Unlock()

	done := make(chan struct{})
	go func() { defer close(done); q.wg.Wait() }()

	select {
	case <-ctx.Done():
		q.logger.Warn("shutdown interrupted by context")
	case <-done:
		q.logger.Info("queue drained, shutdown complete")
	}
}
