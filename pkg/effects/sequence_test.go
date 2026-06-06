package effects

import (
	"context"
	"errors"
	"reflect"
	"testing"
	"time"
)

func TestRunSequenceRunsEffectsInOrder(t *testing.T) {
	renderer := &recordingRenderer{}
	first := &finiteRunnerEffect{frames: []Frame{{Colors: []Color{color(10)}, Width: 1, Height: 1}}}
	second := &finiteRunnerEffect{frames: []Frame{{Colors: []Color{color(20)}, Width: 1, Height: 1}}}

	err := RunSequence(context.Background(), renderer,
		RunConfig{Effect: first, Step: time.Nanosecond},
		RunConfig{Effect: second, Step: time.Nanosecond},
	)
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
}

func TestRunSequenceUsesDefaultStep(t *testing.T) {
	effect := &finiteRunnerEffect{frames: []Frame{{}}}

	err := RunSequence(context.Background(), &recordingRenderer{}, RunConfig{Effect: effect})
	if err != nil {
		t.Fatal(err)
	}

	want := []time.Duration{DefaultRunStep, DefaultRunStep}
	if !reflect.DeepEqual(effect.steps, want) {
		t.Fatalf("steps = %#v, want %#v", effect.steps, want)
	}
}

func TestRunSequenceTreatsRunDurationAsSuccess(t *testing.T) {
	err := RunSequence(context.Background(), &recordingRenderer{},
		RunConfig{
			Effect:   NewSolid(SolidConfig{Color: color(10)}),
			Duration: time.Nanosecond,
			Step:     time.Nanosecond,
		},
		RunConfig{
			Effect: &finiteRunnerEffect{frames: []Frame{{}}},
			Step:   time.Nanosecond,
		},
	)
	if err != nil {
		t.Fatal(err)
	}
}

func TestRunSequenceReturnsParentCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	err := RunSequence(ctx, &recordingRenderer{}, RunConfig{
		Effect: NewSolid(SolidConfig{Color: color(10)}),
		Step:   time.Nanosecond,
	})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("error = %v, want context.Canceled", err)
	}
}
