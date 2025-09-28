package ocr

import (
	"bytes"
	"context"
	"log/slog"
	"os/exec"
	"strings"
	"time"
)

// Runner lets us stub external commands in tests.
type Runner interface {
	Run(ctx context.Context, name string, args ...string) (stdout, stderr []byte, err error)
}

type execRunner struct{}

func (execRunner) Run(ctx context.Context, name string, args ...string) ([]byte, []byte, error) {
	start := time.Now()

	cmd := exec.CommandContext(ctx, name, args...)
	var out, errb bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &errb

	err := cmd.Run()
	dur := time.Since(start)

	if err != nil {
		slog.Error("exec failed",
			"cmd", name,
			"args", strings.Join(args, " "),
			"duration_ms", dur.Milliseconds(),
			"error", err,
			"stderr", truncate(errb.String(), 8<<10), // cap at 8KB
		)
	} else {
		slog.Debug("exec ok",
			"cmd", name,
			"args", strings.Join(args, " "),
			"duration_ms", dur.Milliseconds(),
			"stdout_bytes", out.Len(),
			"stderr_bytes", errb.Len(),
		)
	}

	return out.Bytes(), errb.Bytes(), err
}

func truncate(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max] + "...(truncated)"
}
