package effects

import (
	"math"
	"time"

	"github.com/alessio-palumbo/lifxlan-go/pkg/device"
)

const (
	defaultCometHeadSize = 1
	defaultCometTailSize = 4
	defaultCometFloor    = 0.22
)

// CometConfig configures a Comet effect.
type CometConfig struct {
	Capabilities Capabilities
	Palette      Palette
	// Axis is the axis the comet travels along. Empty uses FlowAxisHorizontal.
	Axis FlowAxis
	// Period is how long one full traversal takes when advanced by Next. Zero uses
	// defaultFlowPeriod. Ignored by FrameAtPhase.
	Period time.Duration
	// HeadSize is the number of logical cells kept at full head brightness. Zero
	// uses defaultCometHeadSize.
	HeadSize int
	// TailSize is the number of logical cells fading behind the head. Zero uses
	// defaultCometTailSize.
	TailSize int
	// Floor is how lit the background stays as a fraction of palette brightness.
	// Zero uses defaultCometFloor.
	Floor float64
}

// Comet moves a bright head with a fading tail over a dimmed background.
type Comet struct {
	cfg     CometConfig
	elapsed time.Duration
}

// NewComet returns a Comet effect.
func NewComet(cfg CometConfig) *Comet {
	if cfg.Period <= 0 {
		cfg.Period = defaultFlowPeriod
	}
	if cfg.HeadSize <= 0 {
		cfg.HeadSize = defaultCometHeadSize
	}
	if cfg.TailSize <= 0 {
		cfg.TailSize = defaultCometTailSize
	}
	if cfg.Floor <= 0 {
		cfg.Floor = defaultCometFloor
	}
	if cfg.Axis == "" {
		cfg.Axis = FlowAxisHorizontal
	}
	return &Comet{cfg: cfg}
}

// Next advances the effect by dt and returns the frame at the new position.
func (c *Comet) Next(dt time.Duration) (Frame, bool) {
	c.elapsed += dt
	phase := float64(c.elapsed) / float64(c.cfg.Period)
	return c.FrameAtPhase(phase, dt), true
}

// FrameAtPhase returns the frame at an absolute position in the comet cycle.
// Whole phases address the same point, and negative phases wrap.
func (c *Comet) FrameAtPhase(phase float64, duration time.Duration) Frame {
	width, height := frameDimensions(c.cfg.Capabilities)
	axis := c.axis(height)
	span := flowSpan(axis, width, height)
	size := FrameSize(width, height)

	head := phase * float64(span)
	colors := make([]Color, 0, size)
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			position := flowPosition(axis, x, y)
			colors = append(colors, c.colorAt(head, position, span))
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
func (c *Comet) Reset() {
	c.elapsed = 0
}

func (c *Comet) colorAt(head float64, position, span int) Color {
	behind := math.Mod(head-float64(position), float64(span))
	if behind < 0 {
		behind += float64(span)
	}

	if behind < float64(c.cfg.HeadSize) {
		return c.cfg.Palette.Accent()
	}
	if behind < float64(c.cfg.HeadSize+c.cfg.TailSize) {
		tailPosition := behind - float64(c.cfg.HeadSize) + 1
		level := c.cfg.Floor + (1-c.cfg.Floor)*(1-tailPosition/float64(c.cfg.TailSize+1))
		color := c.cfg.Palette.Accent()
		color.Brightness = scaleBrightness(color.Brightness, level)
		return color
	}

	color := c.cfg.Palette.Background()
	color.Brightness = scaleBrightness(color.Brightness, c.cfg.Floor)
	return color
}

// axis resolves the configured axis against the surface. Travelling along y on a
// single row would leave the whole surface in unison, which is not an effect.
func (c *Comet) axis(height int) FlowAxis {
	if height <= 1 {
		return FlowAxisHorizontal
	}
	return c.cfg.Axis
}

func cometLightTypes() []device.LightType {
	return []device.LightType{device.LightTypeMultiZone, device.LightTypeMatrix}
}

// compile-time checks that Comet satisfies the effect contracts.
var (
	_ Effect      = (*Comet)(nil)
	_ PhaseEffect = (*Comet)(nil)
)
