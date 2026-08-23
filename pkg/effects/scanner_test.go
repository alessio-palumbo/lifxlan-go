package effects

import (
	"math"
	"testing"
	"time"

	"github.com/alessio-palumbo/lifxlan-go/pkg/device"
)

func scannerPalette() Palette {
	return Palette{
		Accents:     []Color{{Hue: 0, Saturation: 100, Brightness: 100, Kelvin: 3500}},
		Backgrounds: []Color{{Hue: 220, Saturation: 100, Brightness: 50, Kelvin: 3500}},
	}
}

func TestScannerDrawsSoftBandAndBackground(t *testing.T) {
	scanner := NewScanner(ScannerConfig{
		Capabilities:               stripCapabilities(9),
		Palette:                    scannerPalette(),
		Width:                      4,
		BackgroundBrightnessFactor: 0.5,
		PeakBrightnessFactor:       1.5,
	})

	frame := scanner.FrameAtPhase(0, time.Second)

	head := frame.Colors[0]
	edge := frame.Colors[2]
	background := frame.Colors[5]

	if head.Hue != 0 || head.Brightness != 100 {
		t.Fatalf("head = %#v, want boosted accent clamped to 100", head)
	}
	if edge.Hue != 0 || !(edge.Brightness >= 50 && edge.Brightness < head.Brightness) {
		t.Fatalf("edge = %#v, want dimmed accent", edge)
	}
	if background.Hue != 220 || background.Brightness != 25 {
		t.Fatalf("background = %#v, want dimmed background", background)
	}
}

func TestScannerDefaultsPreserveBackgroundAndBoostPeak(t *testing.T) {
	scanner := NewScanner(ScannerConfig{
		Capabilities: stripCapabilities(5),
		Palette: Palette{
			Accents:     []Color{{Hue: 0, Saturation: 100, Brightness: 40, Kelvin: 3500}},
			Backgrounds: []Color{{Hue: 220, Saturation: 100, Brightness: 30, Kelvin: 3500}},
		},
		Width: 1,
	})

	frame := scanner.FrameAtPhase(0, time.Second)

	if frame.Colors[0].Brightness != 52 {
		t.Fatalf("peak brightness = %v, want 52", frame.Colors[0].Brightness)
	}
	if frame.Colors[3].Brightness != 30 {
		t.Fatalf("background brightness = %v, want preserved background 30", frame.Colors[3].Brightness)
	}
}

func TestScannerBouncesAtHalfPhase(t *testing.T) {
	scanner := NewScanner(ScannerConfig{
		Capabilities: stripCapabilities(7),
		Palette:      scannerPalette(),
		Width:        1,
	})

	start := scanner.FrameAtPhase(0, time.Second)
	end := scanner.FrameAtPhase(0.5, time.Second)
	back := scanner.FrameAtPhase(1, time.Second)

	if brightestIndex(start) != 0 {
		t.Fatalf("start brightest index = %d, want 0", brightestIndex(start))
	}
	if brightestIndex(end) != 6 {
		t.Fatalf("end brightest index = %d, want 6", brightestIndex(end))
	}
	if !sameFrame(start, back) {
		t.Fatal("whole phase should return to the start")
	}
}

func TestScannerPhaseIsDeterministicAndWrapped(t *testing.T) {
	scanner := NewScanner(ScannerConfig{
		Capabilities: stripCapabilities(10),
		Palette:      scannerPalette(),
		Width:        3,
	})

	start := scanner.FrameAtPhase(0, time.Second)
	later := scanner.FrameAtPhase(0.25, time.Second)
	again := scanner.FrameAtPhase(0.25, time.Second)

	if sameFrame(start, later) {
		t.Fatal("phase did not move the scanner")
	}
	if !sameFrame(later, again) {
		t.Fatal("same phase produced different frames")
	}
	if !sameFrame(scanner.FrameAtPhase(-0.25, time.Second), scanner.FrameAtPhase(0.75, time.Second)) {
		t.Fatal("negative phase should wrap to the equivalent forward position")
	}
}

func TestScannerAxisChangesTravel(t *testing.T) {
	caps := matrixCapabilities(5, 4)
	palette := scannerPalette()

	horizontal := NewScanner(ScannerConfig{Capabilities: caps, Palette: palette, Axis: FlowAxisHorizontal}).FrameAtPhase(0.25, time.Second)
	vertical := NewScanner(ScannerConfig{Capabilities: caps, Palette: palette, Axis: FlowAxisVertical}).FrameAtPhase(0.25, time.Second)
	diagonal := NewScanner(ScannerConfig{Capabilities: caps, Palette: palette, Axis: FlowAxisDiagonal}).FrameAtPhase(0.25, time.Second)

	if sameFrame(horizontal, vertical) {
		t.Fatal("horizontal and vertical scanner produced the same frame")
	}
	if sameFrame(horizontal, diagonal) {
		t.Fatal("horizontal and diagonal scanner produced the same frame")
	}
	if !columnsUniform(horizontal, 5, 4) {
		t.Fatal("horizontal scanner should vary along x only")
	}
	if !rowsUniform(vertical, 5, 4) {
		t.Fatal("vertical scanner should vary along y only")
	}
}

func TestScannerVerticalFallsBackOnSingleRow(t *testing.T) {
	caps := stripCapabilities(10)
	palette := scannerPalette()

	vertical := NewScanner(ScannerConfig{Capabilities: caps, Palette: palette, Axis: FlowAxisVertical}).FrameAtPhase(0.25, time.Second)
	horizontal := NewScanner(ScannerConfig{Capabilities: caps, Palette: palette, Axis: FlowAxisHorizontal}).FrameAtPhase(0.25, time.Second)

	if !sameFrame(vertical, horizontal) {
		t.Fatal("vertical scanner on one row should behave as horizontal")
	}
}

func TestScannerNextAdvancesOverItsPeriod(t *testing.T) {
	scanner := NewScanner(ScannerConfig{
		Capabilities: stripCapabilities(10),
		Palette:      scannerPalette(),
		Period:       time.Second,
	})

	first, ok := scanner.Next(250 * time.Millisecond)
	if !ok {
		t.Fatal("Next reported no frame")
	}
	second, ok := scanner.Next(250 * time.Millisecond)
	if !ok {
		t.Fatal("Next reported no frame")
	}
	if sameFrame(first, second) {
		t.Fatal("Next did not advance")
	}

	scanner.Reset()
	restarted, _ := scanner.Next(250 * time.Millisecond)
	if !sameFrame(first, restarted) {
		t.Fatal("Reset did not return to the start of the cycle")
	}
}

func TestScannerIsRegistered(t *testing.T) {
	effect, err := New(Config{ID: EffectScanner, Params: map[string]any{
		"palette":                      scannerPalette(),
		"axis":                         string(FlowAxisHorizontal),
		"period":                       500 * time.Millisecond,
		"width":                        5,
		"background_brightness_factor": 0.4,
		"peak_brightness_factor":       1.5,
	}}, Capabilities{LightType: device.LightTypeMultiZone, Zones: 8})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	scanner, ok := effect.(*Scanner)
	if !ok {
		t.Fatalf("registry returned %T, want *Scanner", effect)
	}
	if scanner.cfg.Period != 500*time.Millisecond {
		t.Fatalf("period = %s, want 500ms", scanner.cfg.Period)
	}
	if scanner.cfg.Width != 5 || scanner.cfg.BackgroundBrightnessFactor != 0.4 || scanner.cfg.PeakBrightnessFactor != 1.5 {
		t.Fatalf(
			"config = width %v background %v peak %v",
			scanner.cfg.Width,
			scanner.cfg.BackgroundBrightnessFactor,
			scanner.cfg.PeakBrightnessFactor,
		)
	}

	if _, err := New(Config{ID: EffectScanner}, Capabilities{LightType: device.LightTypeSingleZone}); err == nil {
		t.Fatal("scanner should reject single-zone capabilities")
	}
}

func brightestIndex(frame Frame) int {
	index := 0
	brightness := math.Inf(-1)
	for i, color := range frame.Colors {
		if color.Brightness > brightness {
			index = i
			brightness = color.Brightness
		}
	}
	return index
}
