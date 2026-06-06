package effects

import (
	"reflect"
	"testing"
	"time"
)

func TestRenderProducesTimestampedFrames(t *testing.T) {
	effect := &sequenceEffect{limit: 3}

	got := Render(effect, 100*time.Millisecond, time.Second)
	want := []FrameAt{
		{At: 0, Frame: frameWithHue(0, 100*time.Millisecond)},
		{At: 100 * time.Millisecond, Frame: frameWithHue(1, 100*time.Millisecond)},
		{At: 200 * time.Millisecond, Frame: frameWithHue(2, 100*time.Millisecond)},
	}

	if !reflect.DeepEqual(got, want) {
		t.Fatalf("frames = %#v, want %#v", got, want)
	}
}

func TestRenderResetsEffect(t *testing.T) {
	effect := &sequenceEffect{limit: 2}

	first := Render(effect, time.Second, 3*time.Second)
	second := Render(effect, time.Second, 3*time.Second)

	if !reflect.DeepEqual(first, second) {
		t.Fatalf("second render differed after reset:\nfirst=%#v\nsecond=%#v", first, second)
	}
	if effect.resets != 2 {
		t.Fatalf("resets = %d, want 2", effect.resets)
	}
}

func TestRenderStopsAtDuration(t *testing.T) {
	effect := &sequenceEffect{limit: 10}

	got := Render(effect, time.Second, 2500*time.Millisecond)
	if len(got) != 3 {
		t.Fatalf("frames = %d, want 3", len(got))
	}
	if got[2].At != 2*time.Second {
		t.Fatalf("last timestamp = %s, want 2s", got[2].At)
	}
}

func TestRenderStopsWhenEffectEnds(t *testing.T) {
	effect := &sequenceEffect{limit: 1}

	got := Render(effect, time.Second, 5*time.Second)
	if len(got) != 1 {
		t.Fatalf("frames = %d, want 1", len(got))
	}
	if effect.nextCalls != 2 {
		t.Fatalf("Next calls = %d, want 2", effect.nextCalls)
	}
}

func TestRenderRejectsInvalidInputs(t *testing.T) {
	testCases := map[string]struct {
		effect   Effect
		step     time.Duration
		duration time.Duration
	}{
		"nil effect": {
			step:     time.Second,
			duration: time.Second,
		},
		"zero step": {
			effect:   &sequenceEffect{limit: 1},
			duration: time.Second,
		},
		"negative step": {
			effect:   &sequenceEffect{limit: 1},
			step:     -time.Second,
			duration: time.Second,
		},
		"zero duration": {
			effect: &sequenceEffect{limit: 1},
			step:   time.Second,
		},
		"negative duration": {
			effect:   &sequenceEffect{limit: 1},
			step:     time.Second,
			duration: -time.Second,
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			if got := Render(tc.effect, tc.step, tc.duration); got != nil {
				t.Fatalf("Render returned %#v, want nil", got)
			}
		})
	}
}

type sequenceEffect struct {
	index     int
	limit     int
	resets    int
	nextCalls int
}

func (e *sequenceEffect) Next(dt time.Duration) (Frame, bool) {
	e.nextCalls++
	if e.index >= e.limit {
		return Frame{}, false
	}

	frame := frameWithHue(float64(e.index), dt)
	e.index++
	return frame, true
}

func (e *sequenceEffect) Reset() {
	e.index = 0
	e.resets++
}

func frameWithHue(hue float64, duration time.Duration) Frame {
	return Frame{
		Colors: []Color{{
			Hue:        hue,
			Saturation: 100,
			Brightness: 50,
			Kelvin:     3500,
		}},
		Width:    1,
		Height:   1,
		Duration: duration,
	}
}
