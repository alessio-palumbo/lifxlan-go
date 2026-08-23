package effects

import (
	"math"
	"testing"
	"time"

	"github.com/alessio-palumbo/lifxlan-go/pkg/device"
)

func cometPalette() Palette {
	return Palette{
		Accents:     []Color{{Hue: 30, Saturation: 100, Brightness: 100, Kelvin: 3500}},
		Backgrounds: []Color{{Hue: 220, Saturation: 100, Brightness: 50, Kelvin: 3500}},
	}
}

func TestCometDrawsHeadTailAndBackground(t *testing.T) {
	comet := NewComet(CometConfig{
		Capabilities:               stripCapabilities(10),
		Palette:                    cometPalette(),
		HeadSize:                   1,
		TailSize:                   3,
		BackgroundBrightnessFactor: 0.2,
		PeakBrightnessFactor:       1,
	})

	frame := comet.FrameAtPhase(1, time.Second)

	head := frame.Colors[0]
	firstTail := frame.Colors[9]
	lastTail := frame.Colors[7]
	background := frame.Colors[5]

	if head.Hue != 30 || head.Brightness != 100 {
		t.Fatalf("head = %#v, want full accent", head)
	}
	if !(colorDistance(firstTail, head) < colorDistance(lastTail, head)) {
		t.Fatalf("tail should move away from head color: first distance=%v last distance=%v", colorDistance(firstTail, head), colorDistance(lastTail, head))
	}
	if !(colorDistance(lastTail, background) < colorDistance(firstTail, background)) {
		t.Fatalf("tail should move toward background color: first distance=%v last distance=%v", colorDistance(firstTail, background), colorDistance(lastTail, background))
	}
	if !(firstTail.Brightness < head.Brightness && firstTail.Brightness > lastTail.Brightness) {
		t.Fatalf("tail brightnesses head=%v first=%v last=%v; want fading tail", head.Brightness, firstTail.Brightness, lastTail.Brightness)
	}
	if !(firstTail.Saturation < head.Saturation && firstTail.Saturation > lastTail.Saturation) {
		t.Fatalf("tail saturations head=%v first=%v last=%v; want fading saturation", head.Saturation, firstTail.Saturation, lastTail.Saturation)
	}
	if background.Hue != 220 || background.Brightness != 10 {
		t.Fatalf("background = %#v, want dimmed background", background)
	}
}

func TestCometStartsWithHeadOnly(t *testing.T) {
	comet := NewComet(CometConfig{
		Capabilities:               stripCapabilities(10),
		Palette:                    cometPalette(),
		HeadSize:                   1,
		TailSize:                   3,
		BackgroundBrightnessFactor: 0.2,
		PeakBrightnessFactor:       1,
	})

	frame := comet.FrameAtPhase(0, time.Second)
	head := frame.Colors[0]
	background := frame.Colors[5]

	if head.Hue != 30 || head.Brightness != 100 {
		t.Fatalf("head = %#v, want full accent", head)
	}
	for _, index := range []int{7, 8, 9} {
		if frame.Colors[index] != background {
			t.Fatalf("startup cell %d = %#v, want background %#v", index, frame.Colors[index], background)
		}
	}
}

func TestCometTailGrowsDuringFirstTraversalWithoutWrapping(t *testing.T) {
	comet := NewComet(CometConfig{
		Capabilities:               stripCapabilities(10),
		Palette:                    cometPalette(),
		HeadSize:                   1,
		TailSize:                   3,
		BackgroundBrightnessFactor: 0.2,
		PeakBrightnessFactor:       1,
	})

	frame := comet.FrameAtPhase(0.25, time.Second)
	background := comet.backgroundColor()

	if frame.Colors[2] == background {
		t.Fatal("head should have moved into the traversed portion")
	}
	if frame.Colors[1] == background {
		t.Fatal("tail should be visible behind the head")
	}
	if frame.Colors[9] != background {
		t.Fatalf("far end = %#v, want background before first wrap", frame.Colors[9])
	}
}

func TestCometWrapsTailAfterFirstTraversal(t *testing.T) {
	comet := NewComet(CometConfig{
		Capabilities:               stripCapabilities(10),
		Palette:                    cometPalette(),
		HeadSize:                   1,
		TailSize:                   3,
		BackgroundBrightnessFactor: 0.2,
		PeakBrightnessFactor:       1,
	})

	frame := comet.FrameAtPhase(1, time.Second)
	background := frame.Colors[5]

	for _, index := range []int{7, 8, 9} {
		if frame.Colors[index] == background {
			t.Fatalf("steady-state cell %d should contain wrapped tail, got background", index)
		}
	}
}

func TestCometPeakBrightnessCanBoost(t *testing.T) {
	comet := NewComet(CometConfig{
		Capabilities: stripCapabilities(8),
		Palette: Palette{
			Accents:     []Color{{Hue: 30, Saturation: 100, Brightness: 40, Kelvin: 3500}},
			Backgrounds: []Color{{Hue: 220, Saturation: 100, Brightness: 30, Kelvin: 3500}},
		},
		HeadSize:                   1,
		TailSize:                   2,
		BackgroundBrightnessFactor: 1,
		PeakBrightnessFactor:       1.5,
	})

	frame := comet.FrameAtPhase(0, time.Second)

	if frame.Colors[0].Brightness != 60 {
		t.Fatalf("head brightness = %v, want boosted brightness 60", frame.Colors[0].Brightness)
	}
	if frame.Colors[4].Brightness != 30 {
		t.Fatalf("background brightness = %v, want preserved background 30", frame.Colors[4].Brightness)
	}
}

func TestCometDefaultsPreserveBackgroundAndBoostPeak(t *testing.T) {
	comet := NewComet(CometConfig{
		Capabilities: stripCapabilities(8),
		Palette: Palette{
			Accents:     []Color{{Hue: 30, Saturation: 100, Brightness: 40, Kelvin: 3500}},
			Backgrounds: []Color{{Hue: 220, Saturation: 100, Brightness: 30, Kelvin: 3500}},
		},
		HeadSize: 1,
		TailSize: 2,
	})

	frame := comet.FrameAtPhase(0, time.Second)

	if frame.Colors[0].Brightness != 52 {
		t.Fatalf("head brightness = %v, want default boosted brightness 52", frame.Colors[0].Brightness)
	}
	if frame.Colors[4].Brightness != 30 {
		t.Fatalf("background brightness = %v, want preserved background 30", frame.Colors[4].Brightness)
	}
}

func TestCometTailCurveControlsFalloff(t *testing.T) {
	cfg := CometConfig{
		Capabilities:               stripCapabilities(8),
		Palette:                    cometPalette(),
		HeadSize:                   1,
		TailSize:                   3,
		BackgroundBrightnessFactor: 0.2,
		PeakBrightnessFactor:       1,
		TailCurve:                  1,
		TailSaturationFactor:       1,
	}
	linear := NewComet(cfg).FrameAtPhase(1, time.Second)
	cfg.TailCurve = 3
	curved := NewComet(cfg).FrameAtPhase(1, time.Second)

	if !(curved.Colors[7].Brightness < linear.Colors[7].Brightness) {
		t.Fatalf("curved first tail brightness = %v, want below linear %v", curved.Colors[7].Brightness, linear.Colors[7].Brightness)
	}
}

func TestCometTailSaturationFactorControlsTailIntensity(t *testing.T) {
	comet := NewComet(CometConfig{
		Capabilities:               stripCapabilities(8),
		Palette:                    cometPalette(),
		HeadSize:                   1,
		TailSize:                   3,
		BackgroundBrightnessFactor: 1,
		PeakBrightnessFactor:       1.3,
		TailSaturationFactor:       0.2,
	})

	frame := comet.FrameAtPhase(1, time.Second)

	if !(frame.Colors[7].Saturation > frame.Colors[5].Saturation) {
		t.Fatalf("tail saturations first=%v last=%v, want saturation fade", frame.Colors[7].Saturation, frame.Colors[5].Saturation)
	}
}

func TestCometPhaseIsDeterministicAndWrapped(t *testing.T) {
	comet := NewComet(CometConfig{
		Capabilities: stripCapabilities(10),
		Palette:      cometPalette(),
		HeadSize:     1,
		TailSize:     2,
	})

	start := comet.FrameAtPhase(0, time.Second)
	later := comet.FrameAtPhase(0.25, time.Second)
	again := comet.FrameAtPhase(0.25, time.Second)

	if sameFrame(start, later) {
		t.Fatal("phase did not move the comet")
	}
	if !sameFrame(later, again) {
		t.Fatal("same phase produced different frames")
	}
	if sameFrame(start, comet.FrameAtPhase(1, time.Second)) {
		t.Fatal("phase 0 startup should differ from steady-state phase 1")
	}
	if !sameFrame(comet.FrameAtPhase(1, time.Second), comet.FrameAtPhase(2, time.Second)) {
		t.Fatal("completed whole phases should wrap to the same steady-state frame")
	}
	if !sameFrame(comet.FrameAtPhase(-0.25, time.Second), comet.FrameAtPhase(1.75, time.Second)) {
		t.Fatal("negative phase should wrap to the equivalent forward position")
	}
}

func TestCometAxisChangesTravel(t *testing.T) {
	caps := matrixCapabilities(5, 4)
	palette := cometPalette()

	horizontal := NewComet(CometConfig{Capabilities: caps, Palette: palette, Axis: FlowAxisHorizontal, TailSize: 1}).FrameAtPhase(0.25, time.Second)
	vertical := NewComet(CometConfig{Capabilities: caps, Palette: palette, Axis: FlowAxisVertical, TailSize: 1}).FrameAtPhase(0.25, time.Second)
	diagonal := NewComet(CometConfig{Capabilities: caps, Palette: palette, Axis: FlowAxisDiagonal, TailSize: 1}).FrameAtPhase(0.25, time.Second)

	if sameFrame(horizontal, vertical) {
		t.Fatal("horizontal and vertical comet produced the same frame")
	}
	if sameFrame(horizontal, diagonal) {
		t.Fatal("horizontal and diagonal comet produced the same frame")
	}
	if !columnsUniform(horizontal, 5, 4) {
		t.Fatal("horizontal comet should vary along x only")
	}
	if !rowsUniform(vertical, 5, 4) {
		t.Fatal("vertical comet should vary along y only")
	}
}

func TestCometVerticalFallsBackOnSingleRow(t *testing.T) {
	caps := stripCapabilities(10)
	palette := cometPalette()

	vertical := NewComet(CometConfig{Capabilities: caps, Palette: palette, Axis: FlowAxisVertical}).FrameAtPhase(0.25, time.Second)
	horizontal := NewComet(CometConfig{Capabilities: caps, Palette: palette, Axis: FlowAxisHorizontal}).FrameAtPhase(0.25, time.Second)

	if !sameFrame(vertical, horizontal) {
		t.Fatal("vertical comet on one row should behave as horizontal")
	}
}

func TestCometNextAdvancesOverItsPeriod(t *testing.T) {
	comet := NewComet(CometConfig{
		Capabilities: stripCapabilities(10),
		Palette:      cometPalette(),
		Period:       time.Second,
	})

	first, ok := comet.Next(250 * time.Millisecond)
	if !ok {
		t.Fatal("Next reported no frame")
	}
	second, ok := comet.Next(250 * time.Millisecond)
	if !ok {
		t.Fatal("Next reported no frame")
	}
	if sameFrame(first, second) {
		t.Fatal("Next did not advance")
	}

	comet.Reset()
	restarted, _ := comet.Next(250 * time.Millisecond)
	if !sameFrame(first, restarted) {
		t.Fatal("Reset did not return to the start of the cycle")
	}
}

func TestCometIsRegistered(t *testing.T) {
	effect, err := New(Config{ID: EffectComet, Params: map[string]any{
		"palette":                      cometPalette(),
		"axis":                         string(FlowAxisHorizontal),
		"period":                       500 * time.Millisecond,
		"head_size":                    2,
		"tail_size":                    5,
		"background_brightness_factor": 0.4,
		"peak_brightness_factor":       1.5,
		"tail_curve":                   2.5,
		"tail_saturation_factor":       0.3,
	}}, Capabilities{LightType: device.LightTypeMultiZone, Zones: 8})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	comet, ok := effect.(*Comet)
	if !ok {
		t.Fatalf("registry returned %T, want *Comet", effect)
	}
	if comet.cfg.Period != 500*time.Millisecond {
		t.Fatalf("period = %s, want 500ms", comet.cfg.Period)
	}
	if comet.cfg.HeadSize != 2 || comet.cfg.TailSize != 5 ||
		comet.cfg.BackgroundBrightnessFactor != 0.4 || comet.cfg.PeakBrightnessFactor != 1.5 ||
		comet.cfg.TailCurve != 2.5 || comet.cfg.TailSaturationFactor != 0.3 {
		t.Fatalf(
			"config = head %d tail %d background %v peak %v curve %v saturation %v",
			comet.cfg.HeadSize,
			comet.cfg.TailSize,
			comet.cfg.BackgroundBrightnessFactor,
			comet.cfg.PeakBrightnessFactor,
			comet.cfg.TailCurve,
			comet.cfg.TailSaturationFactor,
		)
	}

	if _, err := New(Config{ID: EffectComet}, Capabilities{LightType: device.LightTypeSingleZone}); err == nil {
		t.Fatal("comet should reject single-zone capabilities")
	}
}

func colorDistance(a, b Color) float64 {
	ar, ag, ab := colorToRGB(a)
	br, bg, bb := colorToRGB(b)
	return math.Sqrt(math.Pow(ar-br, 2) + math.Pow(ag-bg, 2) + math.Pow(ab-bb, 2))
}
