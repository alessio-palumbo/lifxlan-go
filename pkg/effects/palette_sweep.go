package effects

import (
	"math"
	"time"

	"github.com/alessio-palumbo/lifxlan-go/pkg/device"
)

const (
	defaultPaletteSweepBandFraction               = 1.0 / 3.0
	defaultPaletteSweepBackgroundBrightnessFactor = 0.22
	defaultPaletteSweepTailBrightnessFactor       = 0.45
)

// PaletteSweepConfig configures a PaletteSweep effect.
type PaletteSweepConfig struct {
	Capabilities Capabilities
	Palette      Palette
	// Axis is the axis the band travels along. Empty uses FlowAxisHorizontal.
	Axis FlowAxis
	// Direction controls the travel direction along Axis. Empty uses
	// FlowDirectionForward.
	Direction FlowDirection
	// Period is how long one full traversal takes when advanced by Next. Zero uses
	// defaultFlowPeriod. Ignored by FrameAtPhase.
	Period time.Duration
	// BandSize is the moving band width in logical cells. If zero, BandFraction is
	// used instead.
	BandSize int
	// BandFraction is the moving band width as a fraction of the surface span.
	// Zero uses defaultPaletteSweepBandFraction.
	BandFraction float64
	// BackgroundBrightnessFactor scales the drifting background gradient as a
	// fraction of palette brightness. Zero uses
	// defaultPaletteSweepBackgroundBrightnessFactor.
	BackgroundBrightnessFactor float64
	// TailBrightnessFactor is the brightness retained at the trailing edge of the
	// band. Zero uses defaultPaletteSweepTailBrightnessFactor.
	TailBrightnessFactor float64
	// Sampling controls whether gradient colors step cell-by-cell or interpolate
	// between adjacent stops. Empty uses FlowSamplingStep.
	Sampling FlowSamplingMode
}

// PaletteSweep moves a multi-color band over a dim drifting gradient background.
type PaletteSweep struct {
	cfg     PaletteSweepConfig
	elapsed time.Duration
}

// NewPaletteSweep returns a PaletteSweep effect.
func NewPaletteSweep(cfg PaletteSweepConfig) *PaletteSweep {
	if cfg.Period <= 0 {
		cfg.Period = defaultFlowPeriod
	}
	if cfg.Axis == "" {
		cfg.Axis = FlowAxisHorizontal
	}
	if cfg.Direction == "" {
		cfg.Direction = FlowDirectionForward
	}
	if cfg.BandFraction <= 0 {
		cfg.BandFraction = defaultPaletteSweepBandFraction
	}
	if cfg.BandFraction > 1 {
		cfg.BandFraction = 1
	}
	if cfg.BackgroundBrightnessFactor <= 0 {
		cfg.BackgroundBrightnessFactor = defaultPaletteSweepBackgroundBrightnessFactor
	}
	if cfg.BackgroundBrightnessFactor > 1 {
		cfg.BackgroundBrightnessFactor = 1
	}
	if cfg.TailBrightnessFactor <= 0 {
		cfg.TailBrightnessFactor = defaultPaletteSweepTailBrightnessFactor
	}
	if cfg.TailBrightnessFactor > 1 {
		cfg.TailBrightnessFactor = 1
	}
	if cfg.Sampling == "" {
		cfg.Sampling = FlowSamplingStep
	}
	return &PaletteSweep{cfg: cfg}
}

// Next advances the effect by dt and returns the frame at the new position.
func (p *PaletteSweep) Next(dt time.Duration) (Frame, bool) {
	p.elapsed += dt
	phase := float64(p.elapsed) / float64(p.cfg.Period)
	return p.FrameAtPhase(phase, dt), true
}

// FrameAtPhase returns the frame at an absolute position in the sweep cycle.
// Whole phases address the same point, and negative phases wrap.
func (p *PaletteSweep) FrameAtPhase(phase float64, duration time.Duration) Frame {
	width, height := frameDimensions(p.cfg.Capabilities)
	axis := p.axis(height)
	span := flowSpan(axis, width, height)
	size := FrameSize(width, height)
	bandSize := p.bandSize(span)

	bandStops := p.cfg.Palette.GradientStops(bandSize)
	if len(bandStops) == 0 {
		bandStops = []Color{p.cfg.Palette.Primary()}
	}
	backgroundStops := p.cfg.Palette.GradientStops(span)
	if len(backgroundStops) == 0 {
		backgroundStops = []Color{p.cfg.Palette.Background()}
	}

	head := paletteSweepMotion(phase, span, p.cfg.Direction)
	colors := make([]Color, 0, size)
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			position := flowPosition(axis, x, y)
			colors = append(colors, p.colorAt(head, position, span, bandSize, bandStops, backgroundStops))
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
func (p *PaletteSweep) Reset() {
	p.elapsed = 0
}

func (p *PaletteSweep) colorAt(head float64, position, span, bandSize int, bandStops, backgroundStops []Color) Color {
	behind := math.Mod(head-float64(position), float64(span))
	if behind < 0 {
		behind += float64(span)
	}

	if behind < float64(bandSize) {
		color := sampleFlowColor(bandStops, behind, p.cfg.Sampling)
		progress := 0.0
		if bandSize > 1 {
			progress = behind / float64(bandSize-1)
			if progress > 1 {
				progress = 1
			}
		}
		factor := 1 - (1-p.cfg.TailBrightnessFactor)*progress
		color.Brightness = scaleBrightness(color.Brightness, factor)
		return color
	}

	color := sampleFlowColor(backgroundStops, float64(position)-head, p.cfg.Sampling)
	color.Brightness = scaleBrightness(color.Brightness, p.cfg.BackgroundBrightnessFactor)
	return color
}

func (p *PaletteSweep) bandSize(span int) int {
	if span <= 1 {
		return 1
	}
	if p.cfg.BandSize > 0 {
		return min(p.cfg.BandSize, span)
	}
	return min(max(2, int(math.Round(float64(span)*p.cfg.BandFraction))), span)
}

// axis resolves the configured axis against the surface. Travelling along y on a
// single row would leave the whole surface in unison, which is not an effect.
func (p *PaletteSweep) axis(height int) FlowAxis {
	if height <= 1 {
		return FlowAxisHorizontal
	}
	return p.cfg.Axis
}

func paletteSweepMotion(phase float64, span int, direction FlowDirection) float64 {
	motion := phase * float64(span)
	if direction == FlowDirectionReverse {
		return -motion
	}
	return motion
}

func paletteSweepLightTypes() []device.LightType {
	return []device.LightType{device.LightTypeMultiZone, device.LightTypeMatrix}
}

// compile-time checks that PaletteSweep satisfies the effect contracts.
var (
	_ Effect      = (*PaletteSweep)(nil)
	_ PhaseEffect = (*PaletteSweep)(nil)
)
