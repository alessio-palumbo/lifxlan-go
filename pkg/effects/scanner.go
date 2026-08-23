package effects

import (
	"math"
	"time"

	"github.com/alessio-palumbo/lifxlan-go/pkg/device"
)

const (
	defaultScannerWidth                      = 3.0
	defaultScannerBackgroundBrightnessFactor = 1.0
	defaultScannerPeakBrightnessFactor       = 1.3
)

// ScannerConfig configures a Scanner effect.
type ScannerConfig struct {
	Capabilities Capabilities
	Palette      Palette
	// Axis is the axis the scanner travels along. Empty uses FlowAxisHorizontal.
	Axis FlowAxis
	// Period is how long one full bounce cycle takes when advanced by Next. Zero
	// uses defaultFlowPeriod. Ignored by FrameAtPhase.
	Period time.Duration
	// Width is the soft band width in logical cells. Zero uses defaultScannerWidth.
	Width float64
	// BackgroundBrightnessFactor scales the background as a fraction of palette
	// brightness. Zero uses defaultScannerBackgroundBrightnessFactor.
	BackgroundBrightnessFactor float64
	// PeakBrightnessFactor scales the scan peak as a fraction of palette
	// brightness. Values above 1 boost the peak, clamped to 100. Zero uses
	// defaultScannerPeakBrightnessFactor.
	PeakBrightnessFactor float64
}

// Scanner moves a soft band back and forth across a multizone or matrix surface.
type Scanner struct {
	cfg     ScannerConfig
	elapsed time.Duration
}

// NewScanner returns a Scanner effect.
func NewScanner(cfg ScannerConfig) *Scanner {
	if cfg.Period <= 0 {
		cfg.Period = defaultFlowPeriod
	}
	if cfg.Width <= 0 {
		cfg.Width = defaultScannerWidth
	}
	if cfg.BackgroundBrightnessFactor <= 0 {
		cfg.BackgroundBrightnessFactor = defaultScannerBackgroundBrightnessFactor
	}
	if cfg.BackgroundBrightnessFactor > 1 {
		cfg.BackgroundBrightnessFactor = 1
	}
	if cfg.PeakBrightnessFactor <= 0 {
		cfg.PeakBrightnessFactor = defaultScannerPeakBrightnessFactor
	}
	if cfg.PeakBrightnessFactor < cfg.BackgroundBrightnessFactor {
		cfg.PeakBrightnessFactor = cfg.BackgroundBrightnessFactor
	}
	if cfg.Axis == "" {
		cfg.Axis = FlowAxisHorizontal
	}
	return &Scanner{cfg: cfg}
}

// Next advances the effect by dt and returns the frame at the new position.
func (s *Scanner) Next(dt time.Duration) (Frame, bool) {
	s.elapsed += dt
	phase := float64(s.elapsed) / float64(s.cfg.Period)
	return s.FrameAtPhase(phase, dt), true
}

// FrameAtPhase returns the frame at an absolute position in the scanner cycle.
// Whole phases address the same point, negative phases wrap, and half phases are
// the opposite end of the bounce.
func (s *Scanner) FrameAtPhase(phase float64, duration time.Duration) Frame {
	width, height := frameDimensions(s.cfg.Capabilities)
	axis := s.axis(height)
	span := flowSpan(axis, width, height)
	size := FrameSize(width, height)

	head := scannerHead(phase, span)
	colors := make([]Color, 0, size)
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			position := flowPosition(axis, x, y)
			colors = append(colors, s.colorAt(head, float64(position)))
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
func (s *Scanner) Reset() {
	s.elapsed = 0
}

func (s *Scanner) colorAt(head, position float64) Color {
	distance := math.Abs(position - head)
	radius := math.Max(s.cfg.Width/2, 0.5)
	if distance > radius {
		background := s.cfg.Palette.Background()
		background.Brightness = scaleBrightness(background.Brightness, s.cfg.BackgroundBrightnessFactor)
		return background
	}

	level := 0.5 * (1 + math.Cos(math.Pi*distance/radius))
	color := s.cfg.Palette.Accent()
	factor := s.cfg.BackgroundBrightnessFactor + (s.cfg.PeakBrightnessFactor-s.cfg.BackgroundBrightnessFactor)*level
	color.Brightness = scaleBrightness(color.Brightness, factor)
	return color
}

// axis resolves the configured axis against the surface. Travelling along y on a
// single row would leave the whole surface in unison, which is not an effect.
func (s *Scanner) axis(height int) FlowAxis {
	if height <= 1 {
		return FlowAxisHorizontal
	}
	return s.cfg.Axis
}

func scannerHead(phase float64, span int) float64 {
	if span <= 1 {
		return 0
	}
	cycle := phase - math.Floor(phase)
	travel := float64(span - 1)
	if cycle < 0.5 {
		return cycle * 2 * travel
	}
	return (1 - (cycle-0.5)*2) * travel
}

func scannerLightTypes() []device.LightType {
	return []device.LightType{device.LightTypeMultiZone, device.LightTypeMatrix}
}

// compile-time checks that Scanner satisfies the effect contracts.
var (
	_ Effect      = (*Scanner)(nil)
	_ PhaseEffect = (*Scanner)(nil)
)
