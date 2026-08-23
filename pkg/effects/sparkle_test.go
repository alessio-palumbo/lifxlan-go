package effects

import (
	"testing"
	"time"

	"github.com/alessio-palumbo/lifxlan-go/pkg/device"
)

func sparklePalette() Palette {
	return Palette{
		Base:        []Color{{Hue: 10, Saturation: 100, Brightness: 100, Kelvin: 3500}},
		Accents:     []Color{{Hue: 80, Saturation: 100, Brightness: 90, Kelvin: 3500}},
		Backgrounds: []Color{{Hue: 220, Saturation: 100, Brightness: 50, Kelvin: 3500}},
	}
}

func TestSparklePhaseIsDeterministicAndWrapped(t *testing.T) {
	sparkle := NewSparkle(SparkleConfig{
		Capabilities: stripCapabilities(32),
		Palette:      sparklePalette(),
		Density:      0.25,
		Seed:         42,
	})

	phase := sparkle.FrameAtPhase(0.35, time.Second)
	again := sparkle.FrameAtPhase(0.35, time.Second)
	other := sparkle.FrameAtPhase(0.55, time.Second)

	if !sameFrame(phase, again) {
		t.Fatal("same phase produced different frames")
	}
	if sameFrame(phase, other) {
		t.Fatal("different phases produced the same frame")
	}
	if !sameFrame(phase, sparkle.FrameAtPhase(1.35, time.Second)) {
		t.Fatal("whole phase should wrap to the same frame")
	}
	if !sameFrame(sparkle.FrameAtPhase(-0.25, time.Second), sparkle.FrameAtPhase(0.75, time.Second)) {
		t.Fatal("negative phase should wrap to the equivalent forward position")
	}
}

func TestSparkleSeedChangesPattern(t *testing.T) {
	first := NewSparkle(SparkleConfig{Capabilities: stripCapabilities(32), Palette: sparklePalette(), Density: 0.25, Seed: 1})
	second := NewSparkle(SparkleConfig{Capabilities: stripCapabilities(32), Palette: sparklePalette(), Density: 0.25, Seed: 2})

	if sameFrame(first.FrameAtPhase(0.4, time.Second), second.FrameAtPhase(0.4, time.Second)) {
		t.Fatal("different seeds produced the same frame")
	}
}

func TestSparkleUsesBackgroundFloorAndFadingSparkles(t *testing.T) {
	sparkle := NewSparkle(SparkleConfig{
		Capabilities: stripCapabilities(64),
		Palette:      sparklePalette(),
		Density:      0.3,
		Decay:        2,
		Floor:        0.2,
		Seed:         9,
	})

	frame := sparkle.FrameAtPhase(0.4, time.Second)
	var background, active int
	for _, color := range frame.Colors {
		switch {
		case color.Hue == 220 && color.Brightness == 10:
			background++
		case color.Hue == 10 || color.Hue == 80:
			active++
			if color.Brightness < device.MinVisibleBrightness || color.Brightness > device.MaxBrightness {
				t.Fatalf("active brightness = %v, want visible percentage", color.Brightness)
			}
		default:
			t.Fatalf("unexpected color %#v", color)
		}
	}
	if background == 0 {
		t.Fatal("expected some dimmed background cells")
	}
	if active == 0 {
		t.Fatal("expected some active sparkle cells")
	}
}

func TestSparkleNextAdvancesOverItsPeriod(t *testing.T) {
	sparkle := NewSparkle(SparkleConfig{
		Capabilities: stripCapabilities(64),
		Palette:      sparklePalette(),
		Density:      0.3,
		Period:       time.Second,
		Seed:         4,
	})

	first, ok := sparkle.Next(250 * time.Millisecond)
	if !ok {
		t.Fatal("Next reported no frame")
	}
	second, ok := sparkle.Next(250 * time.Millisecond)
	if !ok {
		t.Fatal("Next reported no frame")
	}
	if sameFrame(first, second) {
		t.Fatal("Next did not advance")
	}

	sparkle.Reset()
	restarted, _ := sparkle.Next(250 * time.Millisecond)
	if !sameFrame(first, restarted) {
		t.Fatal("Reset did not return to the start of the cycle")
	}
}

func TestSparkleIsRegistered(t *testing.T) {
	effect, err := New(Config{ID: EffectSparkle, Params: map[string]any{
		"palette": sparklePalette(),
		"density": 0.2,
		"decay":   3,
		"floor":   0.4,
		"seed":    99,
		"period":  500 * time.Millisecond,
	}}, Capabilities{LightType: device.LightTypeMultiZone, Zones: 16})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	sparkle, ok := effect.(*Sparkle)
	if !ok {
		t.Fatalf("registry returned %T, want *Sparkle", effect)
	}
	if sparkle.cfg.Density != 0.2 || sparkle.cfg.Decay != 3 || sparkle.cfg.Floor != 0.4 || sparkle.cfg.Seed != 99 {
		t.Fatalf("config = density %v decay %v floor %v seed %v", sparkle.cfg.Density, sparkle.cfg.Decay, sparkle.cfg.Floor, sparkle.cfg.Seed)
	}
	if sparkle.cfg.Period != 500*time.Millisecond {
		t.Fatalf("period = %s, want 500ms", sparkle.cfg.Period)
	}

	if _, err := New(Config{ID: EffectSparkle}, Capabilities{LightType: device.LightTypeSingleZone}); err == nil {
		t.Fatal("sparkle should reject single-zone capabilities")
	}
}
