package effects

import (
	"context"
	"errors"
	"time"
)

var (
	// ErrMissingEffect is returned when a Runner has no effect.
	ErrMissingEffect = errors.New("missing effect")
	// ErrMissingRenderer is returned when a Runner has no renderer.
	ErrMissingRenderer = errors.New("missing renderer")
	// ErrInvalidStep is returned when a Runner has a non-positive step.
	ErrInvalidStep = errors.New("step must be positive")
)

// Renderer renders logical frames to a target-specific destination.
type Renderer interface {
	RenderFrame(context.Context, Frame) error
}

// Runner runs an effect live through a target-bound renderer.
type Runner struct {
	Effect   Effect
	Renderer Renderer
	Step     time.Duration
}

// NewRunner returns a Runner for effect and renderer using step as the fallback frame duration.
func NewRunner(effect Effect, renderer Renderer, step time.Duration) *Runner {
	return &Runner{Effect: effect, Renderer: renderer, Step: step}
}

// Run renders frames until the effect ends, the context is canceled, or rendering fails.
func (r *Runner) Run(ctx context.Context) error {
	if err := r.validate(); err != nil {
		return err
	}
	if ctx == nil {
		ctx = context.Background()
	}

	r.Effect.Reset()
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		frame, ok := r.Effect.Next(r.Step)
		if !ok {
			return nil
		}
		if err := r.Renderer.RenderFrame(ctx, frame); err != nil {
			return err
		}

		wait := frame.Duration
		if wait <= 0 {
			wait = r.Step
		}
		timer := time.NewTimer(wait)
		select {
		case <-ctx.Done():
			if !timer.Stop() {
				select {
				case <-timer.C:
				default:
				}
			}
			return ctx.Err()
		case <-timer.C:
		}
	}
}

func (r *Runner) validate() error {
	switch {
	case r == nil || r.Effect == nil:
		return ErrMissingEffect
	case r.Renderer == nil:
		return ErrMissingRenderer
	case r.Step <= 0:
		return ErrInvalidStep
	default:
		return nil
	}
}
