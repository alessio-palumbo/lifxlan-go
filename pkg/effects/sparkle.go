package effects

import (
	"math"
	"time"

	"github.com/alessio-palumbo/lifxlan-go/pkg/device"
)

const (
	defaultSparkleDensity = 0.12
	defaultSparkleDecay   = 2.0
	defaultSparkleFloor   = 0.18
	defaultSparklePeriod  = 2 * time.Second
)

// SparkleConfig configures a Sparkle effect.
type SparkleConfig struct {
	Capabilities Capabilities
	Palette      Palette
	// Density is the fraction of a cycle where each cell is lit. Zero uses
	// defaultSparkleDensity.
	Density float64
	// Decay controls how quickly each sparkle fades. Larger values fade faster.
	// Zero uses defaultSparkleDecay.
	Decay float64
	// Floor is how lit the background stays as a fraction of palette brightness.
	// Zero uses defaultSparkleFloor.
	Floor float64
	// Seed changes the deterministic sparkle pattern.
	Seed uint64
	// Period is how long one full sparkle cycle takes when advanced by Next. Zero
	// uses defaultSparklePeriod. Ignored by FrameAtPhase.
	Period time.Duration
}

// Sparkle lights deterministic, random-looking cells over a dimmed background.
type Sparkle struct {
	cfg     SparkleConfig
	elapsed time.Duration
}

// NewSparkle returns a Sparkle effect.
func NewSparkle(cfg SparkleConfig) *Sparkle {
	if cfg.Density <= 0 {
		cfg.Density = defaultSparkleDensity
	}
	if cfg.Density > 1 {
		cfg.Density = 1
	}
	if cfg.Decay <= 0 {
		cfg.Decay = defaultSparkleDecay
	}
	if cfg.Floor <= 0 {
		cfg.Floor = defaultSparkleFloor
	}
	if cfg.Floor > 1 {
		cfg.Floor = 1
	}
	if cfg.Period <= 0 {
		cfg.Period = defaultSparklePeriod
	}
	return &Sparkle{cfg: cfg}
}

// Next advances the effect by dt and returns the frame at the new position.
func (s *Sparkle) Next(dt time.Duration) (Frame, bool) {
	s.elapsed += dt
	phase := float64(s.elapsed) / float64(s.cfg.Period)
	return s.FrameAtPhase(phase, dt), true
}

// FrameAtPhase returns the frame at an absolute position in the sparkle cycle.
// Whole phases address the same point, and negative phases wrap.
func (s *Sparkle) FrameAtPhase(phase float64, duration time.Duration) Frame {
	width, height := frameDimensions(s.cfg.Capabilities)
	size := FrameSize(width, height)
	current := wrappedPhase(phase)

	colors := make([]Color, size)
	for i := range colors {
		colors[i] = s.colorAt(i, current)
	}

	return Frame{
		Colors:   colors,
		Width:    width,
		Height:   height,
		Duration: duration,
	}
}

// Reset returns the effect to the start of its cycle.
func (s *Sparkle) Reset() {
	s.elapsed = 0
}

func (s *Sparkle) colorAt(index int, phase float64) Color {
	background := s.cfg.Palette.Background()
	background.Brightness = scaleBrightness(background.Brightness, s.cfg.Floor)

	start := sparkleUnit(s.cfg.Seed, uint64(index), 0)
	age := phase - start
	if age < 0 {
		age += 1
	}
	if age >= s.cfg.Density {
		return background
	}

	progress := age / s.cfg.Density
	level := math.Pow(1-progress, s.cfg.Decay)
	color := sparklePaletteColor(s.cfg.Palette, s.cfg.Seed, index)
	color.Brightness = scaleBrightness(color.Brightness, level)
	return color
}

func sparklePaletteColor(palette Palette, seed uint64, index int) Color {
	colors := make([]Color, 0, len(palette.Base)+len(palette.Accents))
	colors = append(colors, palette.Base...)
	colors = append(colors, palette.Accents...)
	if len(colors) == 0 {
		return palette.Accent()
	}
	return colors[int(sparkleHash(seed, uint64(index), 1)%uint64(len(colors)))]
}

func sparkleUnit(seed, index, salt uint64) float64 {
	return float64(sparkleHash(seed, index, salt)>>11) / (1 << 53)
}

func sparkleHash(seed, index, salt uint64) uint64 {
	x := seed + randomSeedSalt + index*0xbf58476d1ce4e5b9 + salt*0x94d049bb133111eb
	x ^= x >> 30
	x *= 0xbf58476d1ce4e5b9
	x ^= x >> 27
	x *= 0x94d049bb133111eb
	x ^= x >> 31
	return x
}

func sparkleLightTypes() []device.LightType {
	return []device.LightType{device.LightTypeMultiZone, device.LightTypeMatrix}
}

// compile-time checks that Sparkle satisfies the effect contracts.
var (
	_ Effect      = (*Sparkle)(nil)
	_ PhaseEffect = (*Sparkle)(nil)
)
