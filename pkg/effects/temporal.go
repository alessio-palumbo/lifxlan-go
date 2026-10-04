package effects

import (
	"math"
	"time"
)

const (
	// EffectColorCycle identifies a uniform temporal palette cycle.
	EffectColorCycle EffectID = "color_cycle"
	// EffectBreathe identifies a uniform brightness pulse.
	EffectBreathe EffectID = "breathe"
)

// ColorCycleConfig configures uniform color transitions, including single-zone lights.
type ColorCycleConfig struct {
	Capabilities Capabilities
	Palette      Palette
	// Transition is the time spent blending each color to the next. Zero uses 2s.
	Transition time.Duration
	// Hold is the time spent at each palette color before transitioning.
	Hold time.Duration
}

// ColorCycle holds and blends base colors followed by accents over time.
// Every cell receives the same color; it does not require spatial zones.
type ColorCycle struct {
	cfg     ColorCycleConfig
	colors  []Color
	elapsed time.Duration
}

// NewColorCycle returns a looping effect. Nonpositive Transition uses 2s;
// negative Hold is treated as zero. Palette data is copied.
func NewColorCycle(cfg ColorCycleConfig) *ColorCycle {
	if cfg.Transition <= 0 {
		cfg.Transition = 2 * time.Second
	}
	if cfg.Hold < 0 {
		cfg.Hold = 0
	}
	return &ColorCycle{cfg: cfg, colors: paletteColors(cfg.Palette)}
}

// Next advances the cycle by dt; negative dt does not advance time.
func (c *ColorCycle) Next(dt time.Duration) (Frame, bool) {
	if dt > 0 {
		c.elapsed += dt
	}
	period := (float64(c.cfg.Hold) + float64(c.cfg.Transition)) * float64(len(c.colors))
	return c.FrameAtPhase(float64(c.elapsed)/period, dt), true
}

// FrameAtPhase samples the entire palette cycle; whole phases wrap.
func (c *ColorCycle) FrameAtPhase(phase float64, duration time.Duration) Frame {
	position := temporalPhase(phase) * float64(len(c.colors))
	index := int(position)
	fraction := position - float64(index)
	holdFraction := float64(c.cfg.Hold) / (float64(c.cfg.Hold) + float64(c.cfg.Transition))
	blend := math.Max(0, (fraction-holdFraction)/(1-holdFraction))
	color := blendColor(c.colors[index], c.colors[(index+1)%len(c.colors)], blend)
	return fillFrame(c.cfg.Capabilities, color, duration)
}

// Reset returns to the first palette color.
func (c *ColorCycle) Reset() { c.elapsed = 0 }

// BreatheConfig configures a smooth uniform brightness pulse.
type BreatheConfig struct {
	Capabilities Capabilities
	Color        Color
	// Period is a full minimum-to-maximum-to-minimum cycle. Zero uses 4s.
	Period time.Duration
	// MinBrightness and MaxBrightness are absolute percentages, not factors.
	MinBrightness, MaxBrightness float64
}

// Breathe varies brightness without changing hue, saturation, or kelvin.
type Breathe struct {
	cfg     BreatheConfig
	elapsed time.Duration
}

// NewBreathe returns a looping effect. Brightness is clamped and reversed bounds
// are swapped. Nonpositive Period uses 4s. Equal bounds give constant brightness.
func NewBreathe(cfg BreatheConfig) *Breathe {
	if cfg.Period <= 0 {
		cfg.Period = 4 * time.Second
	}
	cfg.MinBrightness = ClampPercent(cfg.MinBrightness)
	cfg.MaxBrightness = ClampPercent(cfg.MaxBrightness)
	if cfg.MinBrightness > cfg.MaxBrightness {
		cfg.MinBrightness, cfg.MaxBrightness = cfg.MaxBrightness, cfg.MinBrightness
	}
	return &Breathe{cfg: cfg}
}

// Next advances the pulse by dt; negative dt does not advance time.
func (b *Breathe) Next(dt time.Duration) (Frame, bool) {
	if dt > 0 {
		b.elapsed += dt
	}
	return b.FrameAtPhase(float64(b.elapsed)/float64(b.cfg.Period), dt), true
}

// FrameAtPhase samples a pulse, starting at minimum brightness.
func (b *Breathe) FrameAtPhase(phase float64, duration time.Duration) Frame {
	factor := (1 - math.Cos(2*math.Pi*temporalPhase(phase))) / 2
	color := WithBrightness(b.cfg.Color, b.cfg.MinBrightness+(b.cfg.MaxBrightness-b.cfg.MinBrightness)*factor)
	return fillFrame(b.cfg.Capabilities, color, duration)
}

// Reset returns to minimum brightness.
func (b *Breathe) Reset() { b.elapsed = 0 }

func temporalPhase(phase float64) float64 {
	if math.IsNaN(phase) || math.IsInf(phase, 0) {
		return 0
	}
	wrapped := phase - math.Floor(phase)
	// Very small negative phases can round to exactly one.
	if wrapped >= 1 {
		return 0
	}
	return wrapped
}

func init() {
	mustRegister(EffectDefinition{
		ID: EffectColorCycle, Label: "Color Cycle", Description: "Hold and smoothly transition uniform palette colors over time.", DeviceKinds: allLightTypes(),
		Params: []ParamDefinition{
			paletteParamDefinition(Palette{Base: []Color{DefaultColor, {Hue: 20, Saturation: 100, Brightness: 100, Kelvin: 3500}}}),
			{Key: "transition", Label: "Transition", Kind: ParamDuration, Default: 2 * time.Second},
			{Key: "hold", Label: "Hold", Kind: ParamDuration, Default: time.Second},
		},
		New: func(config Config, caps Capabilities) (Effect, error) {
			palette, err := PaletteParam(config.Params, "palette")
			if err != nil {
				return nil, err
			}
			transition, err := DurationParam(config.Params, "transition")
			if err != nil {
				return nil, err
			}
			hold, err := DurationParam(config.Params, "hold")
			if err != nil {
				return nil, err
			}
			if transition <= 0 || hold < 0 {
				return nil, ErrInvalidConfig
			}
			return NewColorCycle(ColorCycleConfig{Capabilities: caps, Palette: palette, Transition: transition, Hold: hold}), nil
		},
	})
	mustRegister(EffectDefinition{
		ID: EffectBreathe, Label: "Breathe", Description: "Pulse uniform brightness smoothly without changing color.", DeviceKinds: allLightTypes(),
		Params: []ParamDefinition{
			colorParamDefinition(DefaultColor),
			{Key: "period", Label: "Period", Kind: ParamDuration, Default: 4 * time.Second},
			{Key: "min_brightness", Label: "Minimum brightness", Kind: ParamNumber, Default: 10.0, Min: float64Ptr(0), Max: float64Ptr(100)},
			{Key: "max_brightness", Label: "Maximum brightness", Kind: ParamNumber, Default: 100.0, Min: float64Ptr(0), Max: float64Ptr(100)},
		},
		New: func(config Config, caps Capabilities) (Effect, error) {
			color, err := ColorParam(config.Params, "color")
			if err != nil {
				return nil, err
			}
			period, err := DurationParam(config.Params, "period")
			if err != nil {
				return nil, err
			}
			min, err := NumberParam(config.Params, "min_brightness")
			if err != nil {
				return nil, err
			}
			max, err := NumberParam(config.Params, "max_brightness")
			if err != nil {
				return nil, err
			}
			if period <= 0 || min > max {
				return nil, ErrInvalidConfig
			}
			return NewBreathe(BreatheConfig{Capabilities: caps, Color: color, Period: period, MinBrightness: min, MaxBrightness: max}), nil
		},
	})
}

var (
	_ PhaseEffect = (*ColorCycle)(nil)
	_ PhaseEffect = (*Breathe)(nil)
)
