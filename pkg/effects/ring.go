package effects

import (
	"math"
	"time"

	"github.com/alessio-palumbo/lifxlan-go/pkg/device"
)

const (
	defaultRingPeriod = 2 * time.Second
	defaultRingWidth  = 1.6
	defaultRingFloor  = 0.22
)

// RingConfig configures a Ring effect.
type RingConfig struct {
	Capabilities Capabilities
	Palette      Palette
	// Period is how long one full expansion takes when the effect is advanced by
	// Next. Zero uses defaultRingPeriod. Ignored by FrameAtPhase, which is given a
	// position directly.
	Period time.Duration
	// Width is the ring thickness in logical cells. Zero uses defaultRingWidth.
	Width float64
	// Floor is how lit the background stays, as a fraction of the ring brightness.
	// Zero uses defaultRingFloor.
	Floor float64
}

// Ring expands a soft ring from the center of a matrix surface.
type Ring struct {
	cfg     RingConfig
	elapsed time.Duration
}

// NewRing returns a Ring effect.
func NewRing(cfg RingConfig) *Ring {
	if cfg.Period <= 0 {
		cfg.Period = defaultRingPeriod
	}
	if cfg.Width <= 0 {
		cfg.Width = defaultRingWidth
	}
	if cfg.Floor <= 0 {
		cfg.Floor = defaultRingFloor
	}
	return &Ring{cfg: cfg}
}

// Next advances the effect by dt and returns the frame at the new position.
func (r *Ring) Next(dt time.Duration) (Frame, bool) {
	r.elapsed += dt
	phase := float64(r.elapsed) / float64(r.cfg.Period)
	return r.FrameAtPhase(phase, dt), true
}

// FrameAtPhase returns the frame at an absolute position in the cycle, where whole
// numbers are the same point in the expansion. Phase may be negative and wraps to
// the equivalent forward position.
func (r *Ring) FrameAtPhase(phase float64, duration time.Duration) Frame {
	width, height := frameDimensions(r.cfg.Capabilities)
	colors := make([]Color, 0, width*height)
	if r.cfg.Capabilities.LightType != device.LightTypeMatrix {
		return fillFrame(r.cfg.Capabilities, r.cfg.Palette.Primary(), duration)
	}

	cx := float64(width-1) / 2
	cy := float64(height-1) / 2
	maxRadius := math.Hypot(cx, cy)
	span := maxRadius + r.cfg.Width
	if span <= 0 {
		span = 1
	}
	radius := wrappedPhase(phase) * span

	ringColor := r.cfg.Palette.Accent()
	background := r.cfg.Palette.Background()
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			distance := math.Abs(math.Hypot(float64(x)-cx, float64(y)-cy) - radius)
			if distance >= r.cfg.Width {
				background.Brightness = scaleBrightness(background.Brightness, r.cfg.Floor)
				colors = append(colors, background)
				continue
			}
			level := r.cfg.Floor + (1-r.cfg.Floor)*(1-distance/r.cfg.Width)
			color := ringColor
			color.Brightness = scaleBrightness(color.Brightness, level)
			colors = append(colors, color)
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
func (r *Ring) Reset() {
	r.elapsed = 0
}

func wrappedPhase(phase float64) float64 {
	wrapped := math.Mod(phase, 1)
	if wrapped < 0 {
		wrapped += 1
	}
	return wrapped
}

// compile-time checks that Ring satisfies the effect contracts.
var (
	_ Effect      = (*Ring)(nil)
	_ PhaseEffect = (*Ring)(nil)
)
