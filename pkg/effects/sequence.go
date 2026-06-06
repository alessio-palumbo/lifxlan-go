package effects

import (
	"context"
	"errors"
	"time"
)

const (
	// DefaultRunStep is used by RunSequence when a run does not specify a positive step.
	DefaultRunStep = 100 * time.Millisecond
)

// RunConfig describes one effect run in a live sequence.
type RunConfig struct {
	Effect   Effect
	Duration time.Duration
	Step     time.Duration
}

// RunSequence runs effects in order through renderer.
func RunSequence(ctx context.Context, renderer Renderer, runs ...RunConfig) error {
	if ctx == nil {
		ctx = context.Background()
	}

	for _, run := range runs {
		if err := runOne(ctx, renderer, run); err != nil {
			return err
		}
	}
	return nil
}

func runOne(ctx context.Context, renderer Renderer, run RunConfig) error {
	step := run.Step
	if step <= 0 {
		step = DefaultRunStep
	}

	runCtx := ctx
	cancel := func() {}
	if run.Duration > 0 {
		runCtx, cancel = context.WithTimeout(ctx, run.Duration)
	}
	defer cancel()

	err := NewRunner(run.Effect, renderer, step).Run(runCtx)
	if errors.Is(err, context.DeadlineExceeded) && run.Duration > 0 && ctx.Err() == nil {
		return nil
	}
	return err
}
