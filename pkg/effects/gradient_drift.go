package effects

import (
	"time"

	"github.com/alessio-palumbo/lifxlan-go/pkg/device"
)

// GradientDriftConfig configures a GradientDrift effect.
type GradientDriftConfig struct {
	Capabilities Capabilities
	Palette      Palette
	// Axis is the axis the palette scrolls along. Empty uses FlowAxisHorizontal.
	Axis FlowAxis
	// Period is how long one full drift takes when advanced by Next. Zero uses
	// defaultFlowPeriod. Ignored by FrameAtPhase.
	Period time.Duration
	// Sampling controls whether palette colors step cell-by-cell or interpolate
	// between adjacent stops. Empty uses FlowSamplingStep.
	Sampling FlowSamplingMode
}

// GradientDrift scrolls palette colors across a multizone or matrix surface
// without changing palette brightness.
type GradientDrift struct {
	cfg     GradientDriftConfig
	elapsed time.Duration
}

// NewGradientDrift returns a GradientDrift effect.
func NewGradientDrift(cfg GradientDriftConfig) *GradientDrift {
	if cfg.Period <= 0 {
		cfg.Period = defaultFlowPeriod
	}
	if cfg.Axis == "" {
		cfg.Axis = FlowAxisHorizontal
	}
	if cfg.Sampling == "" {
		cfg.Sampling = FlowSamplingStep
	}
	return &GradientDrift{cfg: cfg}
}

// Next advances the effect by dt and returns the frame at the new position.
func (g *GradientDrift) Next(dt time.Duration) (Frame, bool) {
	g.elapsed += dt
	phase := float64(g.elapsed) / float64(g.cfg.Period)
	return g.FrameAtPhase(phase, dt), true
}

// FrameAtPhase returns the frame at an absolute position in the drift cycle.
// Whole phases address the same palette position, and negative phases wrap.
func (g *GradientDrift) FrameAtPhase(phase float64, duration time.Duration) Frame {
	width, height := frameDimensions(g.cfg.Capabilities)
	axis := g.axis(height)
	span := flowSpan(axis, width, height)
	size := FrameSize(width, height)

	stops := g.cfg.Palette.GradientStops(span)
	if len(stops) == 0 {
		stops = []Color{g.cfg.Palette.Primary()}
	}

	head := phase * float64(span)
	colors := make([]Color, 0, size)
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			position := flowPosition(axis, x, y)
			colors = append(colors, sampleFlowColor(stops, float64(position)+head, g.cfg.Sampling))
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
func (g *GradientDrift) Reset() {
	g.elapsed = 0
}

// axis resolves the configured axis against the surface. Travelling along y on a
// single row would leave the whole surface in unison, which is not an effect.
func (g *GradientDrift) axis(height int) FlowAxis {
	if height <= 1 {
		return FlowAxisHorizontal
	}
	return g.cfg.Axis
}

func gradientDriftLightTypes() []device.LightType {
	return []device.LightType{device.LightTypeMultiZone, device.LightTypeMatrix}
}

// compile-time checks that GradientDrift satisfies the effect contracts.
var (
	_ Effect      = (*GradientDrift)(nil)
	_ PhaseEffect = (*GradientDrift)(nil)
)
