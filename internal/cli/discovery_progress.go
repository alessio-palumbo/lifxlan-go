package cli

import (
	"context"
	"errors"
	"fmt"
	"io"
	"time"
)

// Synchronous terminal-only progress: no background writer to race with output
// or outlive cancellation. The caller disables it for JSON, redirection, and logs.
func waitForDiscovery(ctx context.Context, duration time.Duration, out io.Writer, animate bool) (err error) {
	if !animate {
		return wait(ctx, duration)
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	timer := time.NewTimer(duration)
	defer timer.Stop()
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()
	defer func() {
		if _, clearErr := io.WriteString(out, "\r\x1b[2K"); clearErr != nil {
			if errors.Is(err, context.Canceled) {
				err = nil
			}
			err = errors.Join(err, clearErr)
		}
	}()
	const frames = "|/-\\"
	for frame := 0; ; frame++ {
		if _, err = fmt.Fprintf(out, "\r%c Discovering devices (%s)...", frames[frame%len(frames)], duration); err != nil {
			return err
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-timer.C:
			return nil
		case <-ticker.C:
		}
	}
}
