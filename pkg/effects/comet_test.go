package effects

import (
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
		Capabilities: stripCapabilities(10),
		Palette:      cometPalette(),
		HeadSize:     1,
		TailSize:     3,
		Floor:        0.2,
	})

	frame := comet.FrameAtPhase(0, time.Second)

	head := frame.Colors[0]
	firstTail := frame.Colors[9]
	lastTail := frame.Colors[7]
	background := frame.Colors[5]

	if head.Hue != 30 || head.Brightness != 100 {
		t.Fatalf("head = %#v, want full accent", head)
	}
	if firstTail.Hue != 30 || lastTail.Hue != 30 {
		t.Fatalf("tail hues = %v/%v, want accent hue", firstTail.Hue, lastTail.Hue)
	}
	if !(firstTail.Brightness < head.Brightness && firstTail.Brightness > lastTail.Brightness) {
		t.Fatalf("tail brightnesses head=%v first=%v last=%v; want fading tail", head.Brightness, firstTail.Brightness, lastTail.Brightness)
	}
	if background.Hue != 220 || background.Brightness != 10 {
		t.Fatalf("background = %#v, want dimmed background", background)
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
	if !sameFrame(start, comet.FrameAtPhase(1, time.Second)) {
		t.Fatal("whole phase should wrap to the same frame")
	}
	if !sameFrame(comet.FrameAtPhase(-0.25, time.Second), comet.FrameAtPhase(0.75, time.Second)) {
		t.Fatal("negative phase should wrap to the equivalent forward position")
	}
}

func TestCometAxisChangesTravel(t *testing.T) {
	caps := matrixCapabilities(5, 4)
	palette := cometPalette()

	horizontal := NewComet(CometConfig{Capabilities: caps, Palette: palette, Axis: FlowAxisHorizontal}).FrameAtPhase(0.25, time.Second)
	vertical := NewComet(CometConfig{Capabilities: caps, Palette: palette, Axis: FlowAxisVertical}).FrameAtPhase(0.25, time.Second)
	diagonal := NewComet(CometConfig{Capabilities: caps, Palette: palette, Axis: FlowAxisDiagonal}).FrameAtPhase(0.25, time.Second)

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
		"palette":   cometPalette(),
		"axis":      string(FlowAxisHorizontal),
		"period":    500 * time.Millisecond,
		"head_size": 2,
		"tail_size": 5,
		"floor":     0.4,
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
	if comet.cfg.HeadSize != 2 || comet.cfg.TailSize != 5 || comet.cfg.Floor != 0.4 {
		t.Fatalf("config = head %d tail %d floor %v", comet.cfg.HeadSize, comet.cfg.TailSize, comet.cfg.Floor)
	}

	if _, err := New(Config{ID: EffectComet}, Capabilities{LightType: device.LightTypeSingleZone}); err == nil {
		t.Fatal("comet should reject single-zone capabilities")
	}
}
