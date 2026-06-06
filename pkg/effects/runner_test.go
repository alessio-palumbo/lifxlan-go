package effects

import (
	"context"
	"errors"
	"reflect"
	"testing"
	"time"
)

func TestRunnerRendersFramesUntilEffectEnds(t *testing.T) {
	effect := &finiteRunnerEffect{frames: []Frame{
		{Colors: []Color{color(10)}, Width: 1, Height: 1},
		{Colors: []Color{color(20)}, Width: 1, Height: 1},
	}}
	renderer := &recordingRenderer{}

	err := NewRunner(effect, renderer, time.Nanosecond).Run(context.Background())
	if err != nil {
		t.Fatal(err)
	}

	want := []Frame{
		{Colors: []Color{color(10)}, Width: 1, Height: 1},
		{Colors: []Color{color(20)}, Width: 1, Height: 1},
	}
	if !reflect.DeepEqual(renderer.frames, want) {
		t.Fatalf("frames = %#v, want %#v", renderer.frames, want)
	}
	if effect.resets != 1 {
		t.Fatalf("resets = %d, want 1", effect.resets)
	}
}

func TestRunnerPassesStepToEffect(t *testing.T) {
	effect := &finiteRunnerEffect{frames: []Frame{{}}}
	renderer := &recordingRenderer{}

	err := NewRunner(effect, renderer, 25*time.Nanosecond).Run(context.Background())
	if err != nil {
		t.Fatal(err)
	}

	if got := effect.steps; !reflect.DeepEqual(got, []time.Duration{25 * time.Nanosecond, 25 * time.Nanosecond}) {
		t.Fatalf("steps = %#v, want two 25ns calls", got)
	}
}

func TestRunnerStopsOnContextCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	renderer := &recordingRenderer{cancelAfter: 2, cancel: cancel}

	err := NewRunner(NewSolid(SolidConfig{Color: color(10)}), renderer, time.Nanosecond).Run(ctx)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("error = %v, want context.Canceled", err)
	}
	if len(renderer.frames) != 2 {
		t.Fatalf("frames = %d, want 2", len(renderer.frames))
	}
}

func TestRunnerReturnsRendererError(t *testing.T) {
	wantErr := errors.New("render failed")
	renderer := &recordingRenderer{err: wantErr}

	err := NewRunner(NewSolid(SolidConfig{Color: color(10)}), renderer, time.Nanosecond).Run(context.Background())
	if !errors.Is(err, wantErr) {
		t.Fatalf("error = %v, want %v", err, wantErr)
	}
}

func TestRunnerValidation(t *testing.T) {
	tests := map[string]struct {
		runner *Runner
		want   error
	}{
		"nil runner": {
			want: ErrMissingEffect,
		},
		"missing effect": {
			runner: &Runner{Renderer: &recordingRenderer{}, Step: time.Second},
			want:   ErrMissingEffect,
		},
		"missing renderer": {
			runner: &Runner{Effect: NewSolid(SolidConfig{}), Step: time.Second},
			want:   ErrMissingRenderer,
		},
		"invalid step": {
			runner: &Runner{Effect: NewSolid(SolidConfig{}), Renderer: &recordingRenderer{}},
			want:   ErrInvalidStep,
		},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			err := tt.runner.Run(context.Background())
			if !errors.Is(err, tt.want) {
				t.Fatalf("error = %v, want %v", err, tt.want)
			}
		})
	}
}

type finiteRunnerEffect struct {
	frames []Frame
	index  int
	resets int
	steps  []time.Duration
}

func (e *finiteRunnerEffect) Next(dt time.Duration) (Frame, bool) {
	e.steps = append(e.steps, dt)
	if e.index >= len(e.frames) {
		return Frame{}, false
	}
	frame := e.frames[e.index]
	e.index++
	return frame, true
}

func (e *finiteRunnerEffect) Reset() {
	e.index = 0
	e.resets++
	e.steps = nil
}

type recordingRenderer struct {
	frames      []Frame
	err         error
	cancelAfter int
	cancel      context.CancelFunc
}

func (r *recordingRenderer) RenderFrame(ctx context.Context, frame Frame) error {
	if r.err != nil {
		return r.err
	}
	r.frames = append(r.frames, frame)
	if r.cancelAfter > 0 && len(r.frames) >= r.cancelAfter && r.cancel != nil {
		r.cancel()
	}
	return nil
}
