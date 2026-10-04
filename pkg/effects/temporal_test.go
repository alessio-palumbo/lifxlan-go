package effects

import (
	"math"
	"reflect"
	"testing"
	"time"

	"github.com/alessio-palumbo/lifxlan-go/pkg/device"
)

func TestColorCycleHoldAndCircularBlend(t *testing.T) {
	palette := Palette{Base: []Color{{Hue: 350, Saturation: 100, Brightness: 20, Kelvin: 3000}, {Hue: 10, Saturation: 50, Brightness: 80, Kelvin: 5000}}}
	c := NewColorCycle(ColorCycleConfig{Palette: palette, Hold: time.Second, Transition: time.Second})
	for _, test := range []struct{ phase, hue, brightness float64 }{{0, 350, 20}, {.125, 350, 20}, {.375, 0, 50}, {.5, 10, 80}, {.875, 0, 50}, {1, 350, 20}, {-.5, 10, 80}, {-math.SmallestNonzeroFloat64, 350, 20}} {
		frame := c.FrameAtPhase(test.phase, time.Millisecond)
		got := frame.Colors[0]
		if math.Abs(got.Hue-test.hue) > 1e-9 || math.Abs(got.Brightness-test.brightness) > 1e-9 {
			t.Fatalf("phase %v: %+v", test.phase, got)
		}
	}
	palette.Base[0].Hue = 100
	if c.FrameAtPhase(0, 0).Colors[0].Hue != 350 {
		t.Fatal("palette not copied")
	}
}

func TestTemporalResetAndUniformSurfaces(t *testing.T) {
	for _, caps := range []Capabilities{
		{LightType: device.LightTypeSingleZone, Width: 1, Height: 1, Zones: 1},
		{LightType: device.LightTypeMultiZone, Width: 4, Height: 1, Zones: 4},
		{LightType: device.LightTypeMatrix, Width: 8, Height: 8, Zones: 64},
	} {
		for _, id := range []EffectID{EffectColorCycle, EffectBreathe} {
			effect, err := New(Config{ID: id}, caps)
			if err != nil {
				t.Fatal(err)
			}
			first, _ := effect.Next(time.Second)
			if err := ValidateFrame(first); err != nil {
				t.Fatal(err)
			}
			for _, color := range first.Colors {
				if color != first.Colors[0] {
					t.Fatal("not uniform")
				}
			}
			effect.Next(time.Second)
			effect.Reset()
			again, _ := effect.Next(time.Second)
			if !reflect.DeepEqual(first, again) {
				t.Fatal("reset not deterministic")
			}
		}
	}
}

func TestBreatheBrightnessAndPhase(t *testing.T) {
	color := Color{Hue: 120, Saturation: 70, Brightness: 99, Kelvin: 4000}
	b := NewBreathe(BreatheConfig{Color: color, MinBrightness: 0, MaxBrightness: 80})
	for _, test := range []struct{ phase, brightness float64 }{{0, 0}, {.25, 40}, {.5, 80}, {.75, 40}, {1, 0}, {-.5, 80}, {math.NaN(), 0}} {
		got := b.FrameAtPhase(test.phase, 0).Colors[0]
		if math.Abs(got.Brightness-test.brightness) > 1e-9 || got.Hue != color.Hue || got.Saturation != color.Saturation || got.Kelvin != color.Kelvin {
			t.Fatalf("phase %v: %+v", test.phase, got)
		}
	}
}

func TestTemporalRegistryRejectsInvalidConfig(t *testing.T) {
	for _, config := range []Config{
		{ID: EffectColorCycle, Params: map[string]any{"transition": "0s"}},
		{ID: EffectColorCycle, Params: map[string]any{"hold": "-1s"}},
		{ID: EffectBreathe, Params: map[string]any{"period": "-1s"}},
		{ID: EffectBreathe, Params: map[string]any{"min_brightness": 90, "max_brightness": 10}},
		{ID: EffectBreathe, Params: map[string]any{"max_brightness": 101}},
	} {
		if _, err := New(config, Capabilities{LightType: device.LightTypeSingleZone}); err == nil {
			t.Fatalf("accepted %+v", config)
		}
	}
}
