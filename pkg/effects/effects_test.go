package effects

import (
	"reflect"
	"testing"
	"time"

	"github.com/alessio-palumbo/lifxlan-go/pkg/device"
)

func TestSolidSingleZone(t *testing.T) {
	effect := NewSolid(SolidConfig{Color: color(10)})

	frame, ok := effect.Next(250 * time.Millisecond)
	if !ok {
		t.Fatal("Next returned ok=false")
	}

	want := Frame{
		Colors:   []Color{color(10)},
		Width:    1,
		Height:   1,
		Duration: 250 * time.Millisecond,
	}
	if !reflect.DeepEqual(frame, want) {
		t.Fatalf("frame = %#v, want %#v", frame, want)
	}
}

func TestSolidMultiZone(t *testing.T) {
	effect := NewSolid(SolidConfig{
		Capabilities: Capabilities{LightType: device.LightTypeMultiZone, Zones: 3},
		Color:        color(10),
	})

	frame, ok := effect.Next(time.Second)
	if !ok {
		t.Fatal("Next returned ok=false")
	}

	want := Frame{
		Colors:   []Color{color(10), color(10), color(10)},
		Width:    3,
		Height:   1,
		Duration: time.Second,
	}
	if !reflect.DeepEqual(frame, want) {
		t.Fatalf("frame = %#v, want %#v", frame, want)
	}
}

func TestGradientMatrix(t *testing.T) {
	effect := NewGradient(GradientConfig{
		Capabilities: Capabilities{LightType: device.LightTypeMatrix, Width: 2, Height: 2},
		Palette: Palette{
			Base:        []Color{color(10)},
			Accents:     []Color{color(90)},
			Backgrounds: []Color{color(200)},
		},
	})

	frame, ok := effect.Next(time.Second)
	if !ok {
		t.Fatal("Next returned ok=false")
	}

	want := Frame{
		Colors:   []Color{color(200), color(10), color(90), color(200)},
		Width:    2,
		Height:   2,
		Duration: time.Second,
	}
	if !reflect.DeepEqual(frame, want) {
		t.Fatalf("frame = %#v, want %#v", frame, want)
	}
}

func TestSweepAdvancesAndWraps(t *testing.T) {
	effect := NewSweep(SweepConfig{
		Capabilities: Capabilities{LightType: device.LightTypeMultiZone, Zones: 3},
		Palette: Palette{
			Accents:     []Color{color(90), color(120)},
			Backgrounds: []Color{color(200)},
		},
	})

	got := Render(effect, time.Second, 4*time.Second)
	want := []FrameAt{
		{At: 0, Frame: sweepFrame([]Color{color(90), color(200), color(200)})},
		{At: time.Second, Frame: sweepFrame([]Color{color(200), color(120), color(200)})},
		{At: 2 * time.Second, Frame: sweepFrame([]Color{color(200), color(200), color(90)})},
		{At: 3 * time.Second, Frame: sweepFrame([]Color{color(120), color(200), color(200)})},
	}

	if !reflect.DeepEqual(got, want) {
		t.Fatalf("frames = %#v, want %#v", got, want)
	}
}

func TestSweepReset(t *testing.T) {
	effect := NewSweep(SweepConfig{
		Capabilities: Capabilities{LightType: device.LightTypeMultiZone, Zones: 2},
		Palette: Palette{
			Accents:     []Color{color(90), color(120)},
			Backgrounds: []Color{color(200)},
		},
	})

	first := Render(effect, time.Second, 2*time.Second)
	second := Render(effect, time.Second, 2*time.Second)

	if !reflect.DeepEqual(first, second) {
		t.Fatalf("second render differed after reset:\nfirst=%#v\nsecond=%#v", first, second)
	}
}

func TestFrameDimensionsFallback(t *testing.T) {
	tests := map[string]struct {
		caps Capabilities
		want [2]int
	}{
		"single zone": {
			caps: Capabilities{LightType: device.LightTypeSingleZone},
			want: [2]int{1, 1},
		},
		"empty multizone": {
			caps: Capabilities{LightType: device.LightTypeMultiZone},
			want: [2]int{1, 1},
		},
		"empty matrix": {
			caps: Capabilities{LightType: device.LightTypeMatrix},
			want: [2]int{1, 1},
		},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			width, height := frameDimensions(tt.caps)
			if got := [2]int{width, height}; got != tt.want {
				t.Fatalf("dimensions = %v, want %v", got, tt.want)
			}
		})
	}
}

func sweepFrame(colors []Color) Frame {
	return Frame{
		Colors:   colors,
		Width:    len(colors),
		Height:   1,
		Duration: time.Second,
	}
}
