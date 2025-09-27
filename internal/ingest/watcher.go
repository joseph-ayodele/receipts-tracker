package ingest

import (
	"context"
	"errors"
	"io/fs"
	"log/slog"
	"path/filepath"
	"strings"
	"time"

	"github.com/fsnotify/fsnotify"
)

// Allowed extensions for discovery (lowercase, without '.').
var defaultExts = map[string]struct{}{
	"pdf":  {},
	"jpg":  {},
	"jpeg": {},
	"png":  {},
}

type WatchConfig struct {
	Roots        []string        // directories to watch (recursive)
	AllowedExts  map[string]struct{}
	InitialScan  bool            // if true, walk roots and emit existing files
	Debounce     time.Duration   // coalesce rapid update/rename bursts
}

func StartWatcher(ctx context.Context, cfg WatchConfig) (<-chan string, <-chan error, error) {
	if len(cfg.Roots) == 0 {
		slog.Error("watcher start failed: no roots provided")
		return nil, nil, errors.New("no roots provided")
	}
	if cfg.AllowedExts == nil {
		cfg.AllowedExts = defaultExts
	}
	evCh := make(chan string, 256)
	errCh := make(chan error, 1)

	w, err := fsnotify.NewWatcher()
	if err != nil {
		slog.Error("failed to create fsnotify watcher", "error", err)
		return nil, nil, err
	}

	// Add roots recursively
	addDir := func(root string) error {
		return filepath.WalkDir(root, func(path string, d fs.DirEntry, walkErr error) error {
			if walkErr != nil {
				return walkErr
			}
			if d.IsDir() {
				return w.Add(path)
			}
			if cfg.InitialScan && allowed(path, cfg.AllowedExts) {
				select {
				case evCh <- path:
				default:
				}
			}
			return nil
		})
	}
	for _, r := range cfg.Roots {
		if err := addDir(r); err != nil {
			slog.Error("failed to add root directory", "root", r, "error", err)
			_ = w.Close()
			return nil, nil, err
		}
	}

	go func() {
		defer close(evCh)
		defer close(errCh)
		defer func(w *fsnotify.Watcher) {
			err := w.Close()
			if err != nil {

			}
		}(w)

		var timer *time.Timer
		pending := map[string]struct{}{}

		sendPending := func() {
			for p := range pending {
				select { case evCh <- p: default: }
				delete(pending, p)
			}
		}

		for {
			select {
			case <-ctx.Done():
				return
			case e := <-w.Events:
				// Track new dirs
				if e.Op&fsnotify.Create == fsnotify.Create {
					// If a directory created, start watching it
					// If it's a file, consider for emit below
					if err := tryAddDir(w, e.Name); err != nil {
						slog.Warn("failed to add new directory to watcher", "path", e.Name, "error", err)
					}
				}

				if allowed(e.Name, cfg.AllowedExts) && (e.Op&(fsnotify.Create|fsnotify.Write|fsnotify.Rename)) != 0 {
					pending[e.Name] = struct{}{}
					if cfg.Debounce > 0 {
						if timer != nil {
							timer.Stop()
						}
						timer = time.AfterFunc(cfg.Debounce, sendPending)
					} else {
						sendPending()
					}
				}
			case err := <-w.Errors:
				slog.Error("watcher error", "error", err)
				select {
				case errCh <- err:
				default:
				}
			}
		}
	}()

	return evCh, errCh, nil
}

func allowed(path string, exts map[string]struct{}) bool {
	ext := strings.TrimPrefix(strings.ToLower(filepath.Ext(path)), ".")
	_, ok := exts[ext]
	return ok
}

func tryAddDir(w *fsnotify.Watcher, path string) error {
	// Best-effort: attempt to add; if not a dir, ignore error.
	if err := w.Add(path); err != nil {
		// swallow errors for non-dirs
	}
	return nil
}
