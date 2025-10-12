package async

import (
	"context"
	"time"

	"github.com/google/uuid"
)

// Job is the smallest useful unit. Extend as needed later (profile, trace, retry, etc).
type Job struct {
	FileID      uuid.UUID
	Force       bool // enqueue even if deduplicated
	SubmittedAt time.Time
	TraceID     string
}

type Queue interface {
	Enqueue(ctx context.Context, job Job) error
	Shutdown(ctx context.Context)
}
