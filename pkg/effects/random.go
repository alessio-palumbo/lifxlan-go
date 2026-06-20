package effects

import "math/rand/v2"

const randomSeedSalt = 0x9e3779b97f4a7c15

// Random is a deterministic random source for effects.
//
// Effects that need random-looking output should keep their own Random and
// call Reset from the effect's Reset method.
type Random struct {
	seed uint64
	rand *rand.Rand
}

// NewRandom returns a deterministic random source initialized with seed.
func NewRandom(seed uint64) *Random {
	r := &Random{seed: seed}
	r.Reset()
	return r
}

// Seed returns the seed used by r.
func (r *Random) Seed() uint64 {
	if r == nil {
		return 0
	}
	return r.seed
}

// Reset rewinds r to its initial seed.
func (r *Random) Reset() {
	if r == nil {
		return
	}
	r.rand = rand.New(rand.NewPCG(r.seed, r.seed^randomSeedSalt))
}

// Float64 returns a deterministic value in [0.0, 1.0).
func (r *Random) Float64() float64 {
	if r == nil {
		return 0
	}
	r.ensure()
	return r.rand.Float64()
}

// IntN returns a deterministic integer in [0, n).
//
// If n <= 0, IntN returns 0.
func (r *Random) IntN(n int) int {
	if r == nil || n <= 0 {
		return 0
	}
	r.ensure()
	return r.rand.IntN(n)
}

// Color returns a deterministic color from colors.
//
// If colors is empty, Color returns DefaultColor.
func (r *Random) Color(colors []Color) Color {
	if len(colors) == 0 {
		return DefaultColor
	}
	return colors[r.IntN(len(colors))]
}

// PaletteColor returns a deterministic color from palette colors.
//
// Base colors are considered first, followed by accents and backgrounds. If
// palette does not define any colors, PaletteColor returns DefaultColor.
func (r *Random) PaletteColor(palette Palette) Color {
	colors := make([]Color, 0, len(palette.Base)+len(palette.Accents)+len(palette.Backgrounds))
	colors = append(colors, palette.Base...)
	colors = append(colors, palette.Accents...)
	colors = append(colors, palette.Backgrounds...)
	return r.Color(colors)
}

func (r *Random) ensure() {
	if r.rand == nil {
		r.Reset()
	}
}
