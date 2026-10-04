package effects

import (
	"fmt"
	"math"
	"slices"
	"time"
)

// PatternBreatheConfig configures a brightness envelope over an initial pattern.
// InitialFrame is runtime input, not persisted effect settings. Its dimensions
// define the logical surface; its Duration is ignored.
type PatternBreatheConfig struct {
	InitialFrame Frame
	// Period is a full minimum-to-maximum-to-minimum cycle. Zero uses 4s.
	Period time.Duration
	// Multipliers scale each original brightness. 1 preserves it; 0 is black.
	// Values above 1 are allowed, but resulting brightness is clamped to 100%.
	// Zero bounds intentionally produce a black frame, not default bounds.
	MinMultiplier, MaxMultiplier float64
}

// PatternBreathe pulses an immutable pattern without changing hue, saturation,
// or Kelvin.
// It performs no capture, network I/O, power changes, or restoration.
type PatternBreathe struct {
	initial                      Frame
	period                       time.Duration
	minMultiplier, maxMultiplier float64
	elapsed                      time.Duration
}

// NewPatternBreathe validates and copies the initial frame. Dimensions must be
// positive and match the color count exactly. HSB must be finite and in range;
// Kelvin must be 1500..9000, except all-zero colors are allowed as blank padding.
// Multipliers must be finite, nonnegative, and ordered. Negative Period fails.
func NewPatternBreathe(cfg PatternBreatheConfig) (*PatternBreathe, error) {
	f := cfg.InitialFrame
	// Division avoids overflowing width*height for malformed dimensions.
	if f.Width <= 0 || f.Height <= 0 || len(f.Colors) == 0 ||
		len(f.Colors)%f.Width != 0 || len(f.Colors)/f.Width != f.Height {
		return nil, fmt.Errorf("%w: initial frame dimensions must match color count", ErrInvalidConfig)
	}
	for i, c := range f.Colors {
		if !patternValueInRange(c.Hue, 360) || !patternValueInRange(c.Saturation, 100) ||
			!patternValueInRange(c.Brightness, 100) ||
			(c != (Color{}) && (c.Kelvin < 1500 || c.Kelvin > 9000)) {
			return nil, fmt.Errorf("%w: invalid initial color %d", ErrInvalidConfig, i)
		}
	}
	if !patternValueInRange(cfg.MinMultiplier, math.MaxFloat64) ||
		!patternValueInRange(cfg.MaxMultiplier, math.MaxFloat64) || cfg.MinMultiplier > cfg.MaxMultiplier {
		return nil, fmt.Errorf("%w: multipliers must be finite, nonnegative, and ordered", ErrInvalidConfig)
	}
	if cfg.Period < 0 {
		return nil, fmt.Errorf("%w: period must not be negative", ErrInvalidConfig)
	}
	if cfg.Period == 0 {
		cfg.Period = 4 * time.Second
	}
	f.Colors = slices.Clone(f.Colors)
	f.Duration = 0
	return &PatternBreathe{initial: f, period: cfg.Period, minMultiplier: cfg.MinMultiplier, maxMultiplier: cfg.MaxMultiplier}, nil
}

// Next advances the pulse by dt; negative dt does not advance time.
func (b *PatternBreathe) Next(dt time.Duration) (Frame, bool) {
	if dt > 0 {
		// Keep elapsed within one cycle to avoid long-running duration overflow.
		advance := dt % b.period
		if advance >= b.period-b.elapsed {
			b.elapsed = advance - (b.period - b.elapsed)
		} else {
			b.elapsed += advance
		}
	}
	return b.FrameAtPhase(float64(b.elapsed)/float64(b.period), dt), true
}

// FrameAtPhase samples the envelope, starting at minimum. Each returned frame
// owns its colors. Samples always scale the original, never a previous result.
func (b *PatternBreathe) FrameAtPhase(phase float64, duration time.Duration) Frame {
	factor := breatheEnvelope(phase)
	multiplier := b.minMultiplier + (b.maxMultiplier-b.minMultiplier)*factor
	f := b.initial
	f.Duration = duration
	f.Colors = make([]Color, len(b.initial.Colors))
	for i, c := range b.initial.Colors {
		f.Colors[i] = WithBrightness(c, c.Brightness*multiplier)
	}
	return f
}

// Reset returns to minimum brightness without recapturing or changing the pattern.
func (b *PatternBreathe) Reset() { b.elapsed = 0 }

func patternValueInRange(v, maximum float64) bool {
	return !math.IsNaN(v) && !math.IsInf(v, 0) && v >= 0 && v <= maximum
}

var _ PhaseEffect = (*PatternBreathe)(nil)
