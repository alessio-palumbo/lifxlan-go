package effects

import (
	"math"
	"testing"
	"time"

	"github.com/alessio-palumbo/lifxlan-go/pkg/device"
)

func flowPalette() Palette {
	return Palette{
		Base:        []Color{{Hue: 300, Saturation: 100, Brightness: 100, Kelvin: 3500}},
		Accents:     []Color{{Hue: 45, Saturation: 100, Brightness: 100, Kelvin: 3500}},
		Backgrounds: []Color{{Hue: 220, Saturation: 100, Brightness: 100, Kelvin: 3500}},
	}
}

func stripCapabilities(zones int) Capabilities {
	return Capabilities{LightType: device.LightTypeMultiZone, Zones: zones, Width: zones, Height: 1}
}

func matrixCapabilities(width, height int) Capabilities {
	return Capabilities{LightType: device.LightTypeMatrix, Width: width, Height: height, Zones: width * height}
}

// The whole surface takes part. Lighting one cell over a flat background, as Sweep
// does, reads as a dot crossing a dead strip rather than as motion.
func TestFlowLightsTheWholeSurface(t *testing.T) {
	flow := NewFlow(FlowConfig{Capabilities: stripCapabilities(16), Palette: flowPalette()})
	frame := flow.FrameAtPhase(0.25, time.Second)

	if len(frame.Colors) != 16 {
		t.Fatalf("colors = %d, want 16", len(frame.Colors))
	}

	var brightest, dimmest float64 = 0, math.MaxFloat64
	hues := map[float64]bool{}
	for _, color := range frame.Colors {
		brightest = math.Max(brightest, color.Brightness)
		dimmest = math.Min(dimmest, color.Brightness)
		hues[color.Hue] = true
	}

	if brightest == dimmest {
		t.Fatal("every cell has the same brightness, so there is no crest")
	}
	if len(hues) < 2 {
		t.Fatalf("only %d hue across the surface", len(hues))
	}
}

// Brightness is treated as 1-100. Driving a powered-on device to 0 leaves it
// looking switched off, which is not what an effect should express.
func TestFlowNeverGoesBelowVisible(t *testing.T) {
	dim := Palette{Base: []Color{{Hue: 300, Saturation: 100, Brightness: 2, Kelvin: 3500}}}
	flow := NewFlow(FlowConfig{Capabilities: stripCapabilities(24), Palette: dim, Floor: 0.01})

	for step := 0; step < 8; step++ {
		frame := flow.FrameAtPhase(float64(step)/8, time.Second)
		for i, color := range frame.Colors {
			if color.Brightness < minVisibleBrightness {
				t.Fatalf("phase %d cell %d has brightness %v, below the visible floor", step, i, color.Brightness)
			}
			if color.Brightness > maxBrightness {
				t.Fatalf("phase %d cell %d has brightness %v, above 100", step, i, color.Brightness)
			}
		}
	}
}

func TestFlowMovesWithPhase(t *testing.T) {
	flow := NewFlow(FlowConfig{Capabilities: stripCapabilities(16), Palette: flowPalette()})

	first := flow.FrameAtPhase(0, time.Second)
	later := flow.FrameAtPhase(0.25, time.Second)
	if sameFrame(first, later) {
		t.Fatal("frame did not change with phase")
	}

	// Whole numbers are the same point in the cycle.
	if !sameFrame(first, flow.FrameAtPhase(1, time.Second)) {
		t.Fatal("phase 1 should return to the start of the cycle")
	}
}

// Timelines built ahead of playback address positions directly and may emit events
// out of order, so the same phase must always give the same frame.
func TestFrameAtPhaseIsDeterministic(t *testing.T) {
	flow := NewFlow(FlowConfig{Capabilities: matrixCapabilities(8, 8), Palette: flowPalette()})

	ahead := flow.FrameAtPhase(0.6, time.Second)
	behind := flow.FrameAtPhase(0.2, time.Second)
	again := flow.FrameAtPhase(0.6, time.Second)

	if sameFrame(ahead, behind) {
		t.Fatal("different phases produced the same frame")
	}
	if !sameFrame(ahead, again) {
		t.Fatal("the same phase produced different frames")
	}
}

// A decreasing phase runs the crest backwards, which is why phase is a signed
// position rather than a step count.
func TestFlowRunsBackwards(t *testing.T) {
	flow := NewFlow(FlowConfig{Capabilities: stripCapabilities(16), Palette: flowPalette()})

	if !sameFrame(flow.FrameAtPhase(-0.25, time.Second), flow.FrameAtPhase(0.75, time.Second)) {
		t.Fatal("negative phase should wrap to the equivalent forward position")
	}
}

func TestFlowAxisChangesTravel(t *testing.T) {
	caps := matrixCapabilities(8, 8)
	palette := flowPalette()

	horizontal := NewFlow(FlowConfig{Capabilities: caps, Palette: palette, Axis: FlowAxisHorizontal}).FrameAtPhase(0.25, time.Second)
	vertical := NewFlow(FlowConfig{Capabilities: caps, Palette: palette, Axis: FlowAxisVertical}).FrameAtPhase(0.25, time.Second)
	diagonal := NewFlow(FlowConfig{Capabilities: caps, Palette: palette, Axis: FlowAxisDiagonal}).FrameAtPhase(0.25, time.Second)

	if sameFrame(horizontal, vertical) {
		t.Fatal("horizontal and vertical travel produced the same frame")
	}
	if sameFrame(horizontal, diagonal) {
		t.Fatal("horizontal and diagonal travel produced the same frame")
	}

	// Horizontal travel leaves every column uniform; vertical leaves every row so.
	if !columnsUniform(horizontal, 8, 8) {
		t.Fatal("horizontal travel should vary along x only")
	}
	if !rowsUniform(vertical, 8, 8) {
		t.Fatal("vertical travel should vary along y only")
	}
}

// A strip has no second row, so travelling along y would leave it in unison.
func TestFlowVerticalFallsBackOnASingleRow(t *testing.T) {
	caps := stripCapabilities(16)
	palette := flowPalette()

	vertical := NewFlow(FlowConfig{Capabilities: caps, Palette: palette, Axis: FlowAxisVertical}).FrameAtPhase(0.25, time.Second)
	horizontal := NewFlow(FlowConfig{Capabilities: caps, Palette: palette, Axis: FlowAxisHorizontal}).FrameAtPhase(0.25, time.Second)

	if !sameFrame(vertical, horizontal) {
		t.Fatal("vertical travel on one row should behave as horizontal")
	}
}

func TestFlowNextAdvancesOverItsPeriod(t *testing.T) {
	flow := NewFlow(FlowConfig{Capabilities: stripCapabilities(16), Palette: flowPalette(), Period: time.Second})

	first, ok := flow.Next(250 * time.Millisecond)
	if !ok {
		t.Fatal("Next reported no frame")
	}
	second, ok := flow.Next(250 * time.Millisecond)
	if !ok {
		t.Fatal("Next reported no frame")
	}
	if sameFrame(first, second) {
		t.Fatal("Next did not advance")
	}

	flow.Reset()
	restarted, _ := flow.Next(250 * time.Millisecond)
	if !sameFrame(first, restarted) {
		t.Fatal("Reset did not return to the start of the cycle")
	}
}

func TestFlowIsRegistered(t *testing.T) {
	effect, err := New(Config{ID: EffectFlow, Params: map[string]any{
		"palette": flowPalette(),
		"axis":    string(FlowAxisDiagonal),
	}}, matrixCapabilities(8, 8))
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	if _, ok := effect.(*Flow); !ok {
		t.Fatalf("registry returned %T, want *Flow", effect)
	}

	if _, err := New(Config{ID: EffectFlow, Params: map[string]any{
		"palette": flowPalette(),
		"axis":    "sideways",
	}}, matrixCapabilities(8, 8)); err == nil {
		t.Fatal("an unknown axis should be rejected")
	}
}

func sameFrame(a, b Frame) bool {
	if len(a.Colors) != len(b.Colors) {
		return false
	}
	for i := range a.Colors {
		if math.Abs(a.Colors[i].Brightness-b.Colors[i].Brightness) > 0.001 ||
			math.Abs(a.Colors[i].Hue-b.Colors[i].Hue) > 0.001 {
			return false
		}
	}
	return true
}

func columnsUniform(frame Frame, width, height int) bool {
	for x := 0; x < width; x++ {
		for y := 1; y < height; y++ {
			if math.Abs(frame.Colors[y*width+x].Brightness-frame.Colors[x].Brightness) > 0.001 {
				return false
			}
		}
	}
	return true
}

func rowsUniform(frame Frame, width, height int) bool {
	for y := 0; y < height; y++ {
		for x := 1; x < width; x++ {
			if math.Abs(frame.Colors[y*width+x].Brightness-frame.Colors[y*width].Brightness) > 0.001 {
				return false
			}
		}
	}
	return true
}
