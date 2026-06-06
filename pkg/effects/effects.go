package effects

import (
	"time"

	"github.com/alessio-palumbo/lifxlan-go/pkg/device"
)

// SolidConfig configures a Solid effect.
type SolidConfig struct {
	Capabilities Capabilities
	Color        Color
}

// Solid fills every logical zone or pixel with the same color.
type Solid struct {
	cfg SolidConfig
}

// NewSolid returns a Solid effect.
func NewSolid(cfg SolidConfig) *Solid {
	return &Solid{cfg: cfg}
}

// Next returns the next solid frame.
func (s *Solid) Next(dt time.Duration) (Frame, bool) {
	return fillFrame(s.cfg.Capabilities, s.cfg.Color, dt), true
}

// Reset resets the effect.
func (s *Solid) Reset() {}

// GradientConfig configures a Gradient effect.
type GradientConfig struct {
	Capabilities Capabilities
	Palette      Palette
}

// Gradient fills the logical surface with deterministic palette stops.
type Gradient struct {
	cfg GradientConfig
}

// NewGradient returns a Gradient effect.
func NewGradient(cfg GradientConfig) *Gradient {
	return &Gradient{cfg: cfg}
}

// Next returns the next gradient frame.
func (g *Gradient) Next(dt time.Duration) (Frame, bool) {
	width, height := frameDimensions(g.cfg.Capabilities)
	return Frame{
		Colors:   g.cfg.Palette.GradientStops(width * height),
		Width:    width,
		Height:   height,
		Duration: dt,
	}, true
}

// Reset resets the effect.
func (g *Gradient) Reset() {}

// SweepConfig configures a Sweep effect.
type SweepConfig struct {
	Capabilities Capabilities
	Palette      Palette
}

// Sweep moves one accent color across a background frame.
type Sweep struct {
	cfg   SweepConfig
	index int
}

// NewSweep returns a Sweep effect.
func NewSweep(cfg SweepConfig) *Sweep {
	return &Sweep{cfg: cfg}
}

// Next returns the next sweep frame.
func (s *Sweep) Next(dt time.Duration) (Frame, bool) {
	width, height := frameDimensions(s.cfg.Capabilities)
	size := width * height
	colors := make([]Color, size)
	for i := range colors {
		colors[i] = s.cfg.Palette.Background()
	}
	colors[wrapIndex(s.index, size)] = s.cfg.Palette.AccentAt(s.index)
	s.index++

	return Frame{
		Colors:   colors,
		Width:    width,
		Height:   height,
		Duration: dt,
	}, true
}

// Reset resets the effect.
func (s *Sweep) Reset() {
	s.index = 0
}

func fillFrame(caps Capabilities, color Color, duration time.Duration) Frame {
	width, height := frameDimensions(caps)
	colors := make([]Color, width*height)
	for i := range colors {
		colors[i] = color
	}
	return Frame{
		Colors:   colors,
		Width:    width,
		Height:   height,
		Duration: duration,
	}
}

func frameDimensions(caps Capabilities) (int, int) {
	switch caps.LightType {
	case device.LightTypeMultiZone:
		return max(caps.Zones, 1), 1
	case device.LightTypeMatrix:
		return max(caps.Width, 1), max(caps.Height, 1)
	default:
		return 1, 1
	}
}
