package effects

import (
	"math"
	"time"

	"github.com/alessio-palumbo/lifxlan-go/pkg/device"
)

const (
	defaultCometHeadSize                   = 1
	defaultCometTailSize                   = 4
	defaultCometBackgroundBrightnessFactor = 1.0
	defaultCometPeakBrightnessFactor       = 1.3
	defaultCometTailCurve                  = 2.0
	defaultCometTailSaturationFactor       = 0.35
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
	// BackgroundBrightnessFactor scales the background as a fraction of palette
	// brightness. Zero uses defaultCometBackgroundBrightnessFactor.
	BackgroundBrightnessFactor float64
	// PeakBrightnessFactor scales the comet head as a fraction of palette
	// brightness. Values above 1 boost the head and tail, clamped to 100. Zero
	// uses defaultCometPeakBrightnessFactor.
	PeakBrightnessFactor float64
	// TailCurve shapes the tail falloff. Higher values drop the tail toward the
	// background faster. Zero uses defaultCometTailCurve.
	TailCurve float64
	// TailSaturationFactor is the saturation retained at the end of the tail, as a
	// fraction of accent saturation. Zero uses defaultCometTailSaturationFactor.
	TailSaturationFactor float64
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
	if cfg.BackgroundBrightnessFactor <= 0 {
		cfg.BackgroundBrightnessFactor = defaultCometBackgroundBrightnessFactor
	}
	if cfg.BackgroundBrightnessFactor > 1 {
		cfg.BackgroundBrightnessFactor = 1
	}
	if cfg.PeakBrightnessFactor <= 0 {
		cfg.PeakBrightnessFactor = defaultCometPeakBrightnessFactor
	}
	if cfg.PeakBrightnessFactor < cfg.BackgroundBrightnessFactor {
		cfg.PeakBrightnessFactor = cfg.BackgroundBrightnessFactor
	}
	if cfg.TailCurve <= 0 {
		cfg.TailCurve = defaultCometTailCurve
	}
	if cfg.TailSaturationFactor <= 0 {
		cfg.TailSaturationFactor = defaultCometTailSaturationFactor
	}
	if cfg.TailSaturationFactor > 1 {
		cfg.TailSaturationFactor = 1
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
// During the first traversal, phase [0,1), the tail does not wrap ahead of the
// head. Later phases use steady-state wrapping, so a completed loop can trail
// across the cycle boundary.
func (c *Comet) FrameAtPhase(phase float64, duration time.Duration) Frame {
	width, height := frameDimensions(c.cfg.Capabilities)
	axis := c.axis(height)
	span := flowSpan(axis, width, height)
	size := FrameSize(width, height)

	head := phase * float64(span)
	wrapTail := phase < 0 || phase >= 1
	colors := make([]Color, 0, size)
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			position := flowPosition(axis, x, y)
			colors = append(colors, c.colorAt(head, position, span, wrapTail))
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

func (c *Comet) colorAt(head float64, position, span int, wrapTail bool) Color {
	behind := head - float64(position)
	if wrapTail {
		behind = math.Mod(behind, float64(span))
		if behind < 0 {
			behind += float64(span)
		}
	} else if behind < 0 {
		return c.backgroundColor()
	}

	if behind < float64(c.cfg.HeadSize) {
		return c.headColor()
	}
	if behind < float64(c.cfg.HeadSize+c.cfg.TailSize) {
		tailPosition := behind - float64(c.cfg.HeadSize) + 1
		tailProgress := tailPosition / float64(c.cfg.TailSize+1)
		tailLevel := math.Pow(1-tailProgress, c.cfg.TailCurve)
		saturationFactor := c.cfg.TailSaturationFactor + (1-c.cfg.TailSaturationFactor)*tailLevel
		color := blendCometColor(c.backgroundColor(), c.headColor(), tailLevel)
		color.Saturation = ClampPercent(color.Saturation * saturationFactor)
		return color
	}

	return c.backgroundColor()
}

func (c *Comet) headColor() Color {
	color := c.cfg.Palette.Accent()
	color.Brightness = scaleBrightness(color.Brightness, c.cfg.PeakBrightnessFactor)
	return color
}

func (c *Comet) backgroundColor() Color {
	color := c.cfg.Palette.Background()
	color.Brightness = scaleBrightness(color.Brightness, c.cfg.BackgroundBrightnessFactor)
	return color
}

func blendCometColor(background, head Color, headLevel float64) Color {
	headLevel = ClampPercent(headLevel*100) / 100
	br, bg, bb := colorToRGB(background)
	hr, hg, hb := colorToRGB(head)
	color := rgbToColor(
		br+(hr-br)*headLevel,
		bg+(hg-bg)*headLevel,
		bb+(hb-bb)*headLevel,
	)
	color.Kelvin = uint16(math.Round(float64(background.Kelvin) + (float64(head.Kelvin)-float64(background.Kelvin))*headLevel))
	return color
}

func colorToRGB(color Color) (float64, float64, float64) {
	hue := math.Mod(color.Hue, 360)
	if hue < 0 {
		hue += 360
	}
	saturation := ClampPercent(color.Saturation) / 100
	value := ClampPercent(color.Brightness) / 100
	chroma := value * saturation
	x := chroma * (1 - math.Abs(math.Mod(hue/60, 2)-1))
	m := value - chroma

	var r, g, b float64
	switch {
	case hue < 60:
		r, g, b = chroma, x, 0
	case hue < 120:
		r, g, b = x, chroma, 0
	case hue < 180:
		r, g, b = 0, chroma, x
	case hue < 240:
		r, g, b = 0, x, chroma
	case hue < 300:
		r, g, b = x, 0, chroma
	default:
		r, g, b = chroma, 0, x
	}

	return r + m, g + m, b + m
}

func rgbToColor(r, g, b float64) Color {
	maxValue := math.Max(r, math.Max(g, b))
	minValue := math.Min(r, math.Min(g, b))
	chroma := maxValue - minValue

	hue := 0.0
	if chroma != 0 {
		switch maxValue {
		case r:
			hue = 60 * math.Mod((g-b)/chroma, 6)
		case g:
			hue = 60 * ((b-r)/chroma + 2)
		default:
			hue = 60 * ((r-g)/chroma + 4)
		}
	}
	if hue < 0 {
		hue += 360
	}

	saturation := 0.0
	if maxValue != 0 {
		saturation = chroma / maxValue
	}

	return Color{
		Hue:        hue,
		Saturation: ClampPercent(saturation * 100),
		Brightness: ClampPercent(maxValue * 100),
	}
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
