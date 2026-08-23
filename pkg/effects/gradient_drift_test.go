package effects

import (
	"math"
	"testing"
	"time"

	"github.com/alessio-palumbo/lifxlan-go/pkg/device"
)

func gradientDriftPalette() Palette {
	return Palette{
		Base: []Color{
			{Hue: 0, Saturation: 100, Brightness: 25, Kelvin: 3500},
			{Hue: 120, Saturation: 100, Brightness: 50, Kelvin: 3500},
			{Hue: 240, Saturation: 100, Brightness: 75, Kelvin: 3500},
		},
	}
}

func TestGradientDriftPreservesPaletteBrightness(t *testing.T) {
	drift := NewGradientDrift(GradientDriftConfig{
		Capabilities: stripCapabilities(12),
		Palette:      gradientDriftPalette(),
	})

	frame := drift.FrameAtPhase(0.25, time.Second)
	stops := gradientDriftPalette().GradientStops(12)
	wantBrightness := make(map[float64]bool, len(stops))
	for _, stop := range stops {
		wantBrightness[stop.Brightness] = true
	}
	for i, color := range frame.Colors {
		if !wantBrightness[color.Brightness] {
			t.Fatalf("cell %d brightness = %v, want a palette gradient stop brightness", i, color.Brightness)
		}
	}
}

func TestGradientDriftPhaseIsDeterministicAndWrapped(t *testing.T) {
	drift := NewGradientDrift(GradientDriftConfig{
		Capabilities: stripCapabilities(10),
		Palette:      gradientDriftPalette(),
	})

	start := drift.FrameAtPhase(0, time.Second)
	later := drift.FrameAtPhase(0.25, time.Second)
	again := drift.FrameAtPhase(0.25, time.Second)

	if sameFrame(start, later) {
		t.Fatal("phase did not move the gradient")
	}
	if !sameFrame(later, again) {
		t.Fatal("same phase produced different frames")
	}
	if !sameFrame(start, drift.FrameAtPhase(1, time.Second)) {
		t.Fatal("whole phase should wrap to the same frame")
	}
	if !sameFrame(drift.FrameAtPhase(-0.25, time.Second), drift.FrameAtPhase(0.75, time.Second)) {
		t.Fatal("negative phase should wrap to the equivalent forward position")
	}
}

func TestGradientDriftAxisChangesTravel(t *testing.T) {
	caps := matrixCapabilities(5, 4)
	palette := gradientDriftPalette()

	horizontal := NewGradientDrift(GradientDriftConfig{Capabilities: caps, Palette: palette, Axis: FlowAxisHorizontal}).FrameAtPhase(0.25, time.Second)
	vertical := NewGradientDrift(GradientDriftConfig{Capabilities: caps, Palette: palette, Axis: FlowAxisVertical}).FrameAtPhase(0.25, time.Second)
	diagonal := NewGradientDrift(GradientDriftConfig{Capabilities: caps, Palette: palette, Axis: FlowAxisDiagonal}).FrameAtPhase(0.25, time.Second)

	if sameFrame(horizontal, vertical) {
		t.Fatal("horizontal and vertical drift produced the same frame")
	}
	if sameFrame(horizontal, diagonal) {
		t.Fatal("horizontal and diagonal drift produced the same frame")
	}
	if !columnsUniformByHue(horizontal, 5, 4) {
		t.Fatal("horizontal drift should vary along x only")
	}
	if !rowsUniformByHue(vertical, 5, 4) {
		t.Fatal("vertical drift should vary along y only")
	}
}

func TestGradientDriftVerticalFallsBackOnSingleRow(t *testing.T) {
	caps := stripCapabilities(12)
	palette := gradientDriftPalette()

	vertical := NewGradientDrift(GradientDriftConfig{Capabilities: caps, Palette: palette, Axis: FlowAxisVertical}).FrameAtPhase(0.25, time.Second)
	horizontal := NewGradientDrift(GradientDriftConfig{Capabilities: caps, Palette: palette, Axis: FlowAxisHorizontal}).FrameAtPhase(0.25, time.Second)

	if !sameFrame(vertical, horizontal) {
		t.Fatal("vertical drift on one row should behave as horizontal")
	}
}

func TestGradientDriftNextAdvancesOverItsPeriod(t *testing.T) {
	drift := NewGradientDrift(GradientDriftConfig{
		Capabilities: stripCapabilities(10),
		Palette:      gradientDriftPalette(),
		Period:       time.Second,
	})

	first, ok := drift.Next(250 * time.Millisecond)
	if !ok {
		t.Fatal("Next reported no frame")
	}
	second, ok := drift.Next(250 * time.Millisecond)
	if !ok {
		t.Fatal("Next reported no frame")
	}
	if sameFrame(first, second) {
		t.Fatal("Next did not advance")
	}

	drift.Reset()
	restarted, _ := drift.Next(250 * time.Millisecond)
	if !sameFrame(first, restarted) {
		t.Fatal("Reset did not return to the start of the cycle")
	}
}

func TestGradientDriftIsRegistered(t *testing.T) {
	effect, err := New(Config{ID: EffectGradientDrift, Params: map[string]any{
		"palette": gradientDriftPalette(),
		"axis":    string(FlowAxisDiagonal),
		"period":  500 * time.Millisecond,
	}}, Capabilities{LightType: device.LightTypeMultiZone, Zones: 8})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	drift, ok := effect.(*GradientDrift)
	if !ok {
		t.Fatalf("registry returned %T, want *GradientDrift", effect)
	}
	if drift.cfg.Axis != FlowAxisDiagonal {
		t.Fatalf("axis = %q, want %q", drift.cfg.Axis, FlowAxisDiagonal)
	}
	if drift.cfg.Period != 500*time.Millisecond {
		t.Fatalf("period = %s, want 500ms", drift.cfg.Period)
	}

	if _, err := New(Config{ID: EffectGradientDrift}, Capabilities{LightType: device.LightTypeSingleZone}); err == nil {
		t.Fatal("gradient drift should reject single-zone capabilities")
	}
}

func columnsUniformByHue(frame Frame, width, height int) bool {
	for x := 0; x < width; x++ {
		for y := 1; y < height; y++ {
			if math.Abs(frame.Colors[y*width+x].Hue-frame.Colors[x].Hue) > 0.001 {
				return false
			}
		}
	}
	return true
}

func rowsUniformByHue(frame Frame, width, height int) bool {
	for y := 0; y < height; y++ {
		for x := 1; x < width; x++ {
			if math.Abs(frame.Colors[y*width+x].Hue-frame.Colors[y*width].Hue) > 0.001 {
				return false
			}
		}
	}
	return true
}
