package effects

// DefaultColor is used when a palette does not define a requested color.
var DefaultColor = Color{Hue: 220, Saturation: 100, Brightness: 100, Kelvin: 3500}

var blankColor = Color{Hue: DefaultColor.Hue, Saturation: DefaultColor.Saturation, Brightness: 0, Kelvin: DefaultColor.Kelvin}

// Palette groups colors for deterministic effects.
type Palette struct {
	Name        string
	Base        []Color
	Accents     []Color
	Backgrounds []Color
}

// Primary returns the first base color, or DefaultColor when no base color is set.
func (p Palette) Primary() Color {
	return firstOr(p.Base, DefaultColor)
}

// Secondary returns the second base color, falling back to Primary.
func (p Palette) Secondary() Color {
	if len(p.Base) > 1 {
		return p.Base[1]
	}
	return p.Primary()
}

// Accent returns the first accent color, falling back to Primary.
func (p Palette) Accent() Color {
	return firstOr(p.Accents, p.Primary())
}

// Background returns the first background color, falling back to Secondary.
func (p Palette) Background() Color {
	return firstOr(p.Backgrounds, p.Secondary())
}

// ColorAt returns a deterministic color for index from base colors followed by accents.
func (p Palette) ColorAt(index int) Color {
	colors := make([]Color, 0, len(p.Base)+len(p.Accents))
	colors = append(colors, p.Base...)
	colors = append(colors, p.Accents...)
	return colorAt(colors, index, p.Primary())
}

// AccentAt returns a deterministic accent for index, falling back to ColorAt.
func (p Palette) AccentAt(index int) Color {
	return colorAt(p.Accents, index, p.ColorAt(index))
}

// GradientStops returns count deterministic stops from backgrounds, base colors, then accents.
func (p Palette) GradientStops(count int) []Color {
	if count <= 0 {
		return nil
	}

	source := make([]Color, 0, len(p.Backgrounds)+len(p.Base)+len(p.Accents))
	source = append(source, p.Backgrounds...)
	source = append(source, p.Base...)
	source = append(source, p.Accents...)
	if len(source) == 0 {
		source = []Color{DefaultColor}
	}

	stops := make([]Color, count)
	for i := range stops {
		stops[i] = source[i%len(source)]
	}
	return stops
}

// WithBrightness returns c with brightness clamped to the valid percentage range.
func WithBrightness(c Color, brightness float64) Color {
	c.Brightness = ClampPercent(brightness)
	return c
}

// ClampPercent clamps value to the valid percentage range used by Color fields.
func ClampPercent(value float64) float64 {
	return max(0, min(value, 100))
}

func firstOr(colors []Color, fallback Color) Color {
	if len(colors) == 0 {
		return fallback
	}
	return colors[0]
}

func colorAt(colors []Color, index int, fallback Color) Color {
	if len(colors) == 0 {
		return fallback
	}
	return colors[wrapIndex(index, len(colors))]
}

// wrapIndex maps any integer index into the valid slice range [0, length).
// It assumes length > 0.
func wrapIndex(index, length int) int {
	index %= length
	if index < 0 {
		index += length
	}
	return index
}
