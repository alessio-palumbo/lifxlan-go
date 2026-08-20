package effects

import (
	"math"
	"time"

	"github.com/alessio-palumbo/lifxlan-go/pkg/device"
)

const (
	// defaultFlowPeriod is one full traversal of the surface when driven by Next.
	defaultFlowPeriod = 2 * time.Second
	// defaultFlowFloor keeps the trough of the wave lit. A travelling highlight over
	// a dimmed surface reads as motion; over an unlit one it reads as a lone dot
	// crossing a dead strip.
	defaultFlowFloor = 0.3
	// minVisibleBrightness keeps a powered-on device from being driven to something
	// indistinguishable from off, so brightness is treated as 1-100 rather than
	// 0-100.
	minVisibleBrightness = 1
	maxBrightness        = 100
)

// FlowAxis is the axis a crest travels along. It is deliberately not called a
// direction: Direction already describes inward and outward travel for concentric
// effects, and this chooses an axis rather than a sense along one.
type FlowAxis string

const (
	// FlowAxisHorizontal travels along x, the length of a strip.
	FlowAxisHorizontal FlowAxis = "horizontal"
	// FlowAxisVertical travels along y. A single-row surface has no y to travel, so
	// it behaves as horizontal there.
	FlowAxisVertical FlowAxis = "vertical"
	// FlowAxisDiagonal travels along x+y, crossing a matrix corner to corner.
	FlowAxisDiagonal FlowAxis = "diagonal"
)

// FlowConfig configures a Flow effect.
type FlowConfig struct {
	Capabilities Capabilities
	Palette      Palette
	// Axis is the axis the crest travels along. Empty uses FlowAxisHorizontal.
	Axis FlowAxis
	// Period is how long one full traversal takes when the effect is advanced by
	// Next. Zero uses defaultFlowPeriod. Ignored by FrameAtPhase, which is given a
	// position directly.
	Period time.Duration
	// Floor is how lit the trough stays, as a fraction of the crest. Zero uses
	// defaultFlowFloor.
	Floor float64
}

// Flow travels a brightness crest across the surface while palette colors scroll
// along with it, so every zone or pixel takes part.
//
// Sweep, by contrast, lights a single cell over a flat background. On a long strip
// that reads as a dot rather than motion, and it leaves most of the surface holding
// one color.
type Flow struct {
	cfg     FlowConfig
	elapsed time.Duration
}

// NewFlow returns a Flow effect.
func NewFlow(cfg FlowConfig) *Flow {
	if cfg.Period <= 0 {
		cfg.Period = defaultFlowPeriod
	}
	if cfg.Floor <= 0 {
		cfg.Floor = defaultFlowFloor
	}
	if cfg.Axis == "" {
		cfg.Axis = FlowAxisHorizontal
	}
	return &Flow{cfg: cfg}
}

// Next advances the effect by dt and returns the frame at the new position.
func (f *Flow) Next(dt time.Duration) (Frame, bool) {
	f.elapsed += dt
	phase := float64(f.elapsed) / float64(f.cfg.Period)
	return f.FrameAtPhase(phase, dt), true
}

// FrameAtPhase returns the frame at an absolute position in the cycle, where whole
// numbers are the same point in the traversal. Phase may run backwards: passing a
// decreasing sequence reverses the travel, and negative values are valid.
//
// This exists for callers that build a timeline ahead of playback rather than
// streaming frames. They know the position of every event they are about to emit
// and need the frame for it, which stepping an effect forward cannot express: two
// events at the same moment, or events generated out of order, would each advance
// the effect and drift from the position they were meant to represent.
func (f *Flow) FrameAtPhase(phase float64, duration time.Duration) Frame {
	width, height := frameDimensions(f.cfg.Capabilities)
	axis := f.axis(height)
	span := flowSpan(axis, width, height)
	size := FrameSize(width, height)

	stops := f.cfg.Palette.GradientStops(span)
	if len(stops) == 0 {
		stops = []Color{f.cfg.Palette.Primary()}
	}

	head := phase * float64(span)
	offset := int(math.Floor(head))
	colors := make([]Color, 0, size)
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			position := flowPosition(axis, x, y)

			color := stops[wrapIndex(position+offset, len(stops))]
			color.Brightness = scaleBrightness(color.Brightness, f.level(head, position, span))
			colors = append(colors, color)
		}
	}

	return Frame{
		Colors:   colors,
		Width:    width,
		Height:   height,
		Duration: duration,
	}
}

// Reset returns the effect to the start of its cycle.
func (f *Flow) Reset() {
	f.elapsed = 0
}

// axis resolves the configured axis against the surface. Travelling along y on a
// single row would leave the whole surface in unison, which is not an effect.
func (f *Flow) axis(height int) FlowAxis {
	if height <= 1 {
		return FlowAxisHorizontal
	}
	return f.cfg.Axis
}

// level is the crest: one cosine cycle across the surface, peaking at the head and
// easing to the configured floor opposite it. A plain cosine keeps every cell
// moving; sharpening the peak flattens the trough into a stretch that looks static.
func (f *Flow) level(head float64, position, span int) float64 {
	behind := math.Mod(head-float64(position), float64(span))
	if behind < 0 {
		behind += float64(span)
	}
	crest := 0.5 * (1 + math.Cos(2*math.Pi*behind/float64(span)))
	return f.cfg.Floor + (1-f.cfg.Floor)*crest
}

func flowPosition(axis FlowAxis, x, y int) int {
	switch axis {
	case FlowAxisVertical:
		return y
	case FlowAxisDiagonal:
		return x + y
	default:
		return x
	}
}

// flowSpan is how far the crest travels before repeating.
func flowSpan(axis FlowAxis, width, height int) int {
	switch axis {
	case FlowAxisVertical:
		return max(height, 1)
	case FlowAxisDiagonal:
		return max(width+height-1, 1)
	default:
		return max(width, 1)
	}
}

func scaleBrightness(brightness, level float64) float64 {
	scaled := brightness * level
	if scaled < minVisibleBrightness {
		return minVisibleBrightness
	}
	if scaled > maxBrightness {
		return maxBrightness
	}
	return scaled
}

// flowLightTypes lists the surfaces a travelling crest makes sense on. A single
// zone has nowhere to travel.
func flowLightTypes() []device.LightType {
	return []device.LightType{device.LightTypeMultiZone, device.LightTypeMatrix}
}

// compile-time check that Flow satisfies the Effect contract.
var _ Effect = (*Flow)(nil)
