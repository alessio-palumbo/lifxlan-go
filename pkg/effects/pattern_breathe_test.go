package effects

import (
	"errors"
	"math"
	"reflect"
	"slices"
	"testing"
	"time"

	"github.com/alessio-palumbo/lifxlan-go/pkg/device"
	"github.com/alessio-palumbo/lifxprotocol-go/gen/protocol/packets"
)

func patternFrame() Frame {
	return Frame{Width: 3, Height: 1, Colors: []Color{
		{Hue: 20, Saturation: 80, Brightness: 80, Kelvin: 3500},
		{Hue: 220, Saturation: 60, Brightness: 40, Kelvin: 5000},
		{}, // A legitimate black padding cell.
	}}
}

func TestPatternBreatheEnvelopeAndClamping(t *testing.T) {
	initial := patternFrame()
	b, err := NewPatternBreathe(PatternBreatheConfig{InitialFrame: initial, MinMultiplier: .1, MaxMultiplier: 1})
	if err != nil {
		t.Fatal(err)
	}
	for _, test := range []struct{ phase, multiplier float64 }{
		{0, .1}, {.25, .55}, {.5, 1}, {.75, .55}, {1, .1}, {-.5, 1}, {math.NaN(), .1}, {math.Inf(1), .1},
	} {
		frame := b.FrameAtPhase(test.phase, 25*time.Millisecond)
		if frame.Width != initial.Width || frame.Height != initial.Height || frame.Duration != 25*time.Millisecond {
			t.Fatal(frame)
		}
		for i, c := range frame.Colors {
			original := initial.Colors[i]
			if math.Abs(c.Brightness-original.Brightness*test.multiplier) > 1e-9 || c.Hue != original.Hue || c.Saturation != original.Saturation || c.Kelvin != original.Kelvin {
				t.Fatalf("phase %v color %d: %+v", test.phase, i, c)
			}
		}
	}
	b, err = NewPatternBreathe(PatternBreatheConfig{InitialFrame: initial, MinMultiplier: 2, MaxMultiplier: 2})
	if err != nil {
		t.Fatal(err)
	}
	got := b.FrameAtPhase(.5, 0)
	if got.Colors[0].Brightness != 100 || got.Colors[1].Brightness != 80 || got.Colors[2].Brightness != 0 {
		t.Fatal(got)
	}
	// Finite multipliers can overflow multiplication; outputs must still clamp.
	b, err = NewPatternBreathe(PatternBreatheConfig{InitialFrame: initial, MinMultiplier: math.MaxFloat64, MaxMultiplier: math.MaxFloat64})
	if err != nil {
		t.Fatal(err)
	}
	got = b.FrameAtPhase(0, 0)
	if got.Colors[0].Brightness != 100 || got.Colors[2].Brightness != 0 {
		t.Fatal(got)
	}
}

func TestPatternBreatheOwnershipSamplingAndReset(t *testing.T) {
	initial := patternFrame()
	original := initial
	original.Colors = slices.Clone(initial.Colors)
	b, err := NewPatternBreathe(PatternBreatheConfig{InitialFrame: initial, MinMultiplier: .1, MaxMultiplier: 1})
	if err != nil {
		t.Fatal(err)
	}
	first, _ := b.Next(time.Second)
	for range 100 {
		b.FrameAtPhase(.25, time.Second)
		b.Next(time.Second)
	}
	b.Reset()
	again, _ := b.Next(time.Second)
	if !reflect.DeepEqual(first, again) || !reflect.DeepEqual(initial, original) {
		t.Fatal("sampling or Reset mutated the baseline")
	}
	initial.Colors[0].Hue = 300
	first.Colors[1].Brightness = 0
	got := b.FrameAtPhase(.5, 0)
	if got.Colors[0] != original.Colors[0] || got.Colors[1] != original.Colors[1] {
		t.Fatal("input/output aliases the internal baseline")
	}
	b.Reset()
	negative, _ := b.Next(-time.Second)
	if !reflect.DeepEqual(negative, b.FrameAtPhase(0, -time.Second)) {
		t.Fatal("negative time advanced effect")
	}
	// Huge steps must wrap without overflowing elapsed duration.
	b.Next(time.Duration(math.MaxInt64))
	if b.elapsed < 0 || b.elapsed >= b.period {
		t.Fatal(b.elapsed)
	}
}

func TestPatternBreatheRejectsInvalidInputs(t *testing.T) {
	valid := PatternBreatheConfig{InitialFrame: patternFrame(), MaxMultiplier: 1}
	cases := []struct {
		name   string
		change func(*PatternBreatheConfig)
	}{
		{"empty", func(c *PatternBreatheConfig) { c.InitialFrame = Frame{} }},
		{"zero width", func(c *PatternBreatheConfig) { c.InitialFrame.Width = 0 }},
		{"negative height", func(c *PatternBreatheConfig) { c.InitialFrame.Height = -1 }},
		{"missing colors", func(c *PatternBreatheConfig) { c.InitialFrame.Width = 4 }},
		{"extra colors", func(c *PatternBreatheConfig) { c.InitialFrame.Width = 2 }},
		{"overflow dimensions", func(c *PatternBreatheConfig) { c.InitialFrame.Width = math.MaxInt; c.InitialFrame.Height = math.MaxInt }},
		{"nan hue", func(c *PatternBreatheConfig) { c.InitialFrame.Colors[0].Hue = math.NaN() }},
		{"infinite saturation", func(c *PatternBreatheConfig) { c.InitialFrame.Colors[0].Saturation = math.Inf(1) }},
		{"negative brightness", func(c *PatternBreatheConfig) { c.InitialFrame.Colors[0].Brightness = -1 }},
		{"excess brightness", func(c *PatternBreatheConfig) { c.InitialFrame.Colors[0].Brightness = 101 }},
		{"excess hue", func(c *PatternBreatheConfig) { c.InitialFrame.Colors[0].Hue = 361 }},
		{"bad kelvin", func(c *PatternBreatheConfig) { c.InitialFrame.Colors[0].Kelvin = 9001 }},
		{"nan minimum", func(c *PatternBreatheConfig) { c.MinMultiplier = math.NaN() }},
		{"infinite maximum", func(c *PatternBreatheConfig) { c.MaxMultiplier = math.Inf(1) }},
		{"negative multiplier", func(c *PatternBreatheConfig) { c.MinMultiplier = -1 }},
		{"reversed bounds", func(c *PatternBreatheConfig) { c.MinMultiplier = 2 }},
		{"negative period", func(c *PatternBreatheConfig) { c.Period = -time.Second }},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			cfg := valid
			cfg.InitialFrame.Colors = slices.Clone(valid.InitialFrame.Colors)
			test.change(&cfg)
			if _, err := NewPatternBreathe(cfg); !errors.Is(err, ErrInvalidConfig) {
				t.Fatalf("err=%v", err)
			}
		})
	}
}

func TestPatternBreatheSurfacesAndBlackState(t *testing.T) {
	for _, shape := range [][2]int{{1, 1}, {34, 1}, {8, 8}} {
		initial := NewFrame(shape[0], shape[1], 0, Color{Hue: 120, Saturation: 90, Brightness: 0, Kelvin: 3500})
		b, err := NewPatternBreathe(PatternBreatheConfig{InitialFrame: initial, MaxMultiplier: 1})
		if err != nil {
			t.Fatal(err)
		}
		if !reflect.DeepEqual(b.FrameAtPhase(.5, 0), initial) {
			t.Fatal("black state was changed")
		}
	}
	b, err := NewPatternBreathe(PatternBreatheConfig{InitialFrame: patternFrame()})
	if err != nil {
		t.Fatal(err)
	}
	for _, c := range b.FrameAtPhase(.5, 0).Colors {
		if c.Brightness != 0 {
			t.Fatal("zero bounds were replaced with defaults")
		}
	}
}

func TestPatternBreatheMatrixStateRoundTrip(t *testing.T) {
	dev := device.Device{LightType: device.LightTypeMatrix, MatrixProperties: device.MatrixProperties{
		Width: 2, Height: 2, ChainLength: 2,
		ChainOrientations: []device.Orientation{device.OrientationLeft, device.OrientationRightSideUp},
		ChainZones: [][]packets.LightHsbk{
			deviceColors(color(0), color(1), color(2), color(3)),
			deviceColors(color(4), color(5), color(6), color(7)),
		},
	}}
	initial, err := FrameFromDeviceState(dev, 0)
	if err != nil {
		t.Fatal(err)
	}
	b, err := NewPatternBreathe(PatternBreatheConfig{InitialFrame: initial, MaxMultiplier: 1})
	if err != nil {
		t.Fatal(err)
	}
	frames, err := AdaptFrameToSurface(b.FrameAtPhase(.5, 0), device.SurfaceFromDevice(dev), AdaptOptions{})
	if err != nil {
		t.Fatal(err)
	}
	for _, frame := range frames {
		physical := device.ReorientMatrix(frame.SendWidth, frame.Height, frame.Orientation, frameColorsToDevice(frame.Colors))
		if !reflect.DeepEqual(physical, dev.MatrixProperties.ChainZones[frame.ChainIndex]) {
			t.Fatal("pattern changed after state-to-frame-to-physical round trip")
		}
	}
}
