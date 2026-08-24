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
	// Direction controls the travel direction along Axis. Empty uses
	// FlowDirectionForward.
	Direction FlowDirection
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
	cfg  GradientDriftConfig
	flow *Flow
}

// NewGradientDrift returns a GradientDrift effect.
func NewGradientDrift(cfg GradientDriftConfig) *GradientDrift {
	if cfg.Period <= 0 {
		cfg.Period = defaultFlowPeriod
	}
	if cfg.Axis == "" {
		cfg.Axis = FlowAxisHorizontal
	}
	if cfg.Direction == "" {
		cfg.Direction = FlowDirectionForward
	}
	if cfg.Sampling == "" {
		cfg.Sampling = FlowSamplingStep
	}
	return &GradientDrift{
		cfg: cfg,
		flow: NewFlow(FlowConfig{
			Capabilities:   cfg.Capabilities,
			Palette:        cfg.Palette,
			Axis:           cfg.Axis,
			Direction:      cfg.Direction,
			Period:         cfg.Period,
			BrightnessMode: FlowBrightnessConstant,
			Sampling:       cfg.Sampling,
		}),
	}
}

// Next advances the effect by dt and returns the frame at the new position.
func (g *GradientDrift) Next(dt time.Duration) (Frame, bool) {
	return g.flow.Next(dt)
}

// FrameAtPhase returns the frame at an absolute position in the drift cycle.
// Whole phases address the same palette position, and negative phases wrap.
func (g *GradientDrift) FrameAtPhase(phase float64, duration time.Duration) Frame {
	return g.flow.FrameAtPhase(phase, duration)
}

// Reset returns the effect to the start of its cycle.
func (g *GradientDrift) Reset() {
	g.flow.Reset()
}

func gradientDriftLightTypes() []device.LightType {
	return []device.LightType{device.LightTypeMultiZone, device.LightTypeMatrix}
}

// compile-time checks that GradientDrift satisfies the effect contracts.
var (
	_ Effect      = (*GradientDrift)(nil)
	_ PhaseEffect = (*GradientDrift)(nil)
)
