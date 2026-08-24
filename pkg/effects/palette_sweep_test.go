package effects

import (
	"testing"
	"time"

	"github.com/alessio-palumbo/lifxlan-go/pkg/device"
)

func paletteSweepPalette() Palette {
	return Palette{
		Base: []Color{
			{Hue: 0, Saturation: 100, Brightness: 100, Kelvin: 3500},
			{Hue: 120, Saturation: 100, Brightness: 100, Kelvin: 3500},
			{Hue: 240, Saturation: 100, Brightness: 100, Kelvin: 3500},
		},
		Backgrounds: []Color{{Hue: 300, Saturation: 100, Brightness: 100, Kelvin: 3500}},
	}
}

func TestPaletteSweepDrawsBandOverDimDriftingGradient(t *testing.T) {
	sweep := NewPaletteSweep(PaletteSweepConfig{
		Capabilities:               stripCapabilities(12),
		Palette:                    paletteSweepPalette(),
		BandSize:                   4,
		BackgroundBrightnessFactor: 0.22,
		TailBrightnessFactor:       0.45,
	})

	frame := sweep.FrameAtPhase(1, time.Second)

	head := frame.Colors[0]
	midBand := frame.Colors[10]
	tail := frame.Colors[9]
	background := frame.Colors[6]

	if head.Brightness != 100 {
		t.Fatalf("head brightness = %v, want full palette brightness", head.Brightness)
	}
	if !(midBand.Brightness < head.Brightness && midBand.Brightness > tail.Brightness) {
		t.Fatalf("band brightness head=%v mid=%v tail=%v, want fading band", head.Brightness, midBand.Brightness, tail.Brightness)
	}
	if background.Brightness != 22 {
		t.Fatalf("background brightness = %v, want dimmed background", background.Brightness)
	}

	hues := map[float64]bool{}
	for _, color := range frame.Colors {
		hues[color.Hue] = true
	}
	if len(hues) < 4 {
		t.Fatalf("hues = %v, want multi-color band and background gradient", hues)
	}
}

func TestPaletteSweepDefaultBandSizeUsesSurfaceFraction(t *testing.T) {
	sweep := NewPaletteSweep(PaletteSweepConfig{
		Capabilities: stripCapabilities(12),
		Palette:      paletteSweepPalette(),
	})

	if got := sweep.bandSize(12); got != 4 {
		t.Fatalf("band size = %d, want one third of 12", got)
	}
	if got := sweep.bandSize(5); got != 2 {
		t.Fatalf("small band size = %d, want minimum visible band of 2", got)
	}
}

func TestPaletteSweepPhaseIsDeterministicAndWrapped(t *testing.T) {
	sweep := NewPaletteSweep(PaletteSweepConfig{
		Capabilities: stripCapabilities(12),
		Palette:      paletteSweepPalette(),
	})

	start := sweep.FrameAtPhase(0, time.Second)
	later := sweep.FrameAtPhase(0.25, time.Second)
	again := sweep.FrameAtPhase(0.25, time.Second)

	if sameFrame(start, later) {
		t.Fatal("phase did not move the sweep")
	}
	if !sameFrame(later, again) {
		t.Fatal("same phase produced different frames")
	}
	if !sameFrame(start, sweep.FrameAtPhase(1, time.Second)) {
		t.Fatal("whole phase should wrap to the same frame")
	}
	if !sameFrame(sweep.FrameAtPhase(-0.25, time.Second), sweep.FrameAtPhase(0.75, time.Second)) {
		t.Fatal("negative phase should wrap to the equivalent forward position")
	}
}

func TestPaletteSweepDirectionControlsVisibleMotion(t *testing.T) {
	palette := paletteSweepPalette()
	forward := NewPaletteSweep(PaletteSweepConfig{
		Capabilities: stripCapabilities(12),
		Palette:      palette,
		BandSize:     3,
	})
	reverse := NewPaletteSweep(PaletteSweepConfig{
		Capabilities: stripCapabilities(12),
		Palette:      palette,
		Direction:    FlowDirectionReverse,
		BandSize:     3,
	})

	start := forward.FrameAtPhase(0, time.Second)
	forwardLater := forward.FrameAtPhase(1.0/12.0, time.Second)
	reverseLater := reverse.FrameAtPhase(1.0/12.0, time.Second)

	if got := brightestIndex(start); got != 0 {
		t.Fatalf("start brightest index = %d, want 0", got)
	}
	if got := brightestIndex(forwardLater); got != 1 {
		t.Fatalf("forward brightest index = %d, want 1", got)
	}
	if got := brightestIndex(reverseLater); got != 11 {
		t.Fatalf("reverse brightest index = %d, want 11", got)
	}
}

func TestPaletteSweepAxisChangesTravel(t *testing.T) {
	caps := matrixCapabilities(5, 4)
	palette := paletteSweepPalette()

	horizontal := NewPaletteSweep(PaletteSweepConfig{Capabilities: caps, Palette: palette, Axis: FlowAxisHorizontal}).FrameAtPhase(0.25, time.Second)
	vertical := NewPaletteSweep(PaletteSweepConfig{Capabilities: caps, Palette: palette, Axis: FlowAxisVertical}).FrameAtPhase(0.25, time.Second)
	diagonal := NewPaletteSweep(PaletteSweepConfig{Capabilities: caps, Palette: palette, Axis: FlowAxisDiagonal}).FrameAtPhase(0.25, time.Second)

	if sameFrame(horizontal, vertical) {
		t.Fatal("horizontal and vertical sweep produced the same frame")
	}
	if sameFrame(horizontal, diagonal) {
		t.Fatal("horizontal and diagonal sweep produced the same frame")
	}
	if !columnsUniform(horizontal, 5, 4) {
		t.Fatal("horizontal sweep should vary along x only")
	}
	if !rowsUniform(vertical, 5, 4) {
		t.Fatal("vertical sweep should vary along y only")
	}
}

func TestPaletteSweepVerticalFallsBackOnSingleRow(t *testing.T) {
	caps := stripCapabilities(12)
	palette := paletteSweepPalette()

	vertical := NewPaletteSweep(PaletteSweepConfig{Capabilities: caps, Palette: palette, Axis: FlowAxisVertical}).FrameAtPhase(0.25, time.Second)
	horizontal := NewPaletteSweep(PaletteSweepConfig{Capabilities: caps, Palette: palette, Axis: FlowAxisHorizontal}).FrameAtPhase(0.25, time.Second)

	if !sameFrame(vertical, horizontal) {
		t.Fatal("vertical sweep on one row should behave as horizontal")
	}
}

func TestPaletteSweepNextAdvancesOverItsPeriod(t *testing.T) {
	sweep := NewPaletteSweep(PaletteSweepConfig{
		Capabilities: stripCapabilities(12),
		Palette:      paletteSweepPalette(),
		Period:       time.Second,
	})

	first, ok := sweep.Next(250 * time.Millisecond)
	if !ok {
		t.Fatal("Next reported no frame")
	}
	second, ok := sweep.Next(250 * time.Millisecond)
	if !ok {
		t.Fatal("Next reported no frame")
	}
	if sameFrame(first, second) {
		t.Fatal("Next did not advance")
	}

	sweep.Reset()
	restarted, _ := sweep.Next(250 * time.Millisecond)
	if !sameFrame(first, restarted) {
		t.Fatal("Reset did not return to the start of the cycle")
	}
}

func TestPaletteSweepIsRegistered(t *testing.T) {
	effect, err := New(Config{ID: EffectPaletteSweep, Params: map[string]any{
		"palette":                      paletteSweepPalette(),
		"axis":                         string(FlowAxisHorizontal),
		"direction":                    string(FlowDirectionReverse),
		"period":                       500 * time.Millisecond,
		"band_size":                    5,
		"background_brightness_factor": 0.4,
		"tail_brightness_factor":       0.5,
		"sampling":                     string(FlowSamplingInterpolate),
	}}, Capabilities{LightType: device.LightTypeMultiZone, Zones: 12})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	sweep, ok := effect.(*PaletteSweep)
	if !ok {
		t.Fatalf("registry returned %T, want *PaletteSweep", effect)
	}
	if sweep.cfg.Direction != FlowDirectionReverse {
		t.Fatalf("direction = %q, want %q", sweep.cfg.Direction, FlowDirectionReverse)
	}
	if sweep.cfg.BandSize != 5 || sweep.cfg.BackgroundBrightnessFactor != 0.4 ||
		sweep.cfg.TailBrightnessFactor != 0.5 || sweep.cfg.Sampling != FlowSamplingInterpolate {
		t.Fatalf("config = %#v", sweep.cfg)
	}

	if _, err := New(Config{ID: EffectPaletteSweep}, Capabilities{LightType: device.LightTypeSingleZone}); err == nil {
		t.Fatal("palette sweep should reject single-zone capabilities")
	}
}
