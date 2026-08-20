package effects

import (
	"math"
	"testing"
	"time"

	"github.com/alessio-palumbo/lifxlan-go/pkg/device"
)

func ringPalette() Palette {
	return Palette{
		Base:        []Color{{Hue: 300, Saturation: 100, Brightness: 100, Kelvin: 3500}},
		Accents:     []Color{{Hue: 45, Saturation: 100, Brightness: 100, Kelvin: 3500}},
		Backgrounds: []Color{{Hue: 220, Saturation: 100, Brightness: 100, Kelvin: 3500}},
	}
}

func TestRingExpandsFromCenter(t *testing.T) {
	ring := NewRing(RingConfig{Capabilities: matrixCapabilities(5, 5), Palette: ringPalette()})

	center := ring.FrameAtPhase(0, time.Second)
	later := ring.FrameAtPhase(0.5, time.Second)

	centerIndex := 2*center.Width + 2
	if center.Colors[centerIndex].Hue != 45 {
		t.Fatalf("center hue = %v, want ring accent hue", center.Colors[centerIndex].Hue)
	}
	if center.Colors[centerIndex].Brightness <= center.Colors[0].Brightness {
		t.Fatalf("center brightness = %v, corner brightness = %v; want center ring at phase 0", center.Colors[centerIndex].Brightness, center.Colors[0].Brightness)
	}
	if sameFrame(center, later) {
		t.Fatal("ring did not move with phase")
	}
	if later.Colors[centerIndex].Brightness >= later.Colors[2].Brightness {
		t.Fatalf("later center brightness = %v, edge brightness = %v; want ring away from center", later.Colors[centerIndex].Brightness, later.Colors[2].Brightness)
	}
}

func TestRingPhaseIsDeterministicAndWrapped(t *testing.T) {
	ring := NewRing(RingConfig{Capabilities: matrixCapabilities(5, 5), Palette: ringPalette()})

	phase := ring.FrameAtPhase(0.4, time.Second)
	again := ring.FrameAtPhase(0.4, time.Second)
	if !sameFrame(phase, again) {
		t.Fatal("the same phase produced different frames")
	}
	if !sameFrame(phase, ring.FrameAtPhase(1.4, time.Second)) {
		t.Fatal("whole phases should return to the same point in the cycle")
	}
	if !sameFrame(ring.FrameAtPhase(-0.25, time.Second), ring.FrameAtPhase(0.75, time.Second)) {
		t.Fatal("negative phase should wrap to the equivalent forward position")
	}
}

func TestRingNextAdvancesOverItsPeriod(t *testing.T) {
	ring := NewRing(RingConfig{Capabilities: matrixCapabilities(5, 5), Palette: ringPalette(), Period: time.Second})

	first, ok := ring.Next(250 * time.Millisecond)
	if !ok {
		t.Fatal("Next reported no frame")
	}
	second, ok := ring.Next(250 * time.Millisecond)
	if !ok {
		t.Fatal("Next reported no frame")
	}
	if sameFrame(first, second) {
		t.Fatal("Next did not advance")
	}

	ring.Reset()
	restarted, _ := ring.Next(250 * time.Millisecond)
	if !sameFrame(first, restarted) {
		t.Fatal("Reset did not return to the start of the cycle")
	}
}

func TestRingUsesFloorAndVisibleBrightness(t *testing.T) {
	ring := NewRing(RingConfig{
		Capabilities: matrixCapabilities(5, 5),
		Palette:      Palette{Accents: []Color{{Hue: 45, Saturation: 100, Brightness: 2, Kelvin: 3500}}},
		Floor:        0.01,
	})
	frame := ring.FrameAtPhase(0.25, time.Second)

	for i, color := range frame.Colors {
		if color.Brightness < minVisibleBrightness {
			t.Fatalf("cell %d has brightness %v, below visible floor", i, color.Brightness)
		}
		if color.Brightness > maxBrightness {
			t.Fatalf("cell %d has brightness %v, above 100", i, color.Brightness)
		}
	}
}

func TestRingIsRegistered(t *testing.T) {
	effect, err := New(Config{ID: EffectRing, Params: map[string]any{
		"palette": ringPalette(),
		"period":  500 * time.Millisecond,
		"width":   2.5,
		"floor":   0.4,
	}}, matrixCapabilities(5, 5))
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	ring, ok := effect.(*Ring)
	if !ok {
		t.Fatalf("registry returned %T, want *Ring", effect)
	}
	if ring.cfg.Period != 500*time.Millisecond {
		t.Fatalf("period = %s, want 500ms", ring.cfg.Period)
	}
	if math.Abs(ring.cfg.Width-2.5) > 0.001 {
		t.Fatalf("width = %v, want 2.5", ring.cfg.Width)
	}
	if ring.cfg.Floor != 0.4 {
		t.Fatalf("floor = %v, want 0.4", ring.cfg.Floor)
	}

	if _, err := New(Config{ID: EffectRing}, Capabilities{LightType: device.LightTypeMultiZone, Zones: 8}); err == nil {
		t.Fatal("ring should reject non-matrix capabilities")
	}
	if _, err := New(Config{ID: EffectRing, Params: map[string]any{"floor": 2}}, matrixCapabilities(5, 5)); err == nil {
		t.Fatal("a floor above 1 should be rejected")
	}
}
