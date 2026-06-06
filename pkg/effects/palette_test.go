package effects

import (
	"reflect"
	"testing"
)

func TestPaletteColorAccessors(t *testing.T) {
	palette := testPalette()

	tests := map[string]struct {
		got  Color
		want Color
	}{
		"primary": {
			got:  palette.Primary(),
			want: color(10),
		},
		"secondary": {
			got:  palette.Secondary(),
			want: color(20),
		},
		"accent": {
			got:  palette.Accent(),
			want: color(90),
		},
		"background": {
			got:  palette.Background(),
			want: color(200),
		},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			if tt.got != tt.want {
				t.Fatalf("color = %#v, want %#v", tt.got, tt.want)
			}
		})
	}
}

func TestPaletteFallbacks(t *testing.T) {
	var palette Palette

	if got := palette.Primary(); got != DefaultColor {
		t.Fatalf("primary = %#v, want %#v", got, DefaultColor)
	}
	if got := palette.Secondary(); got != DefaultColor {
		t.Fatalf("secondary = %#v, want %#v", got, DefaultColor)
	}
	if got := palette.Accent(); got != DefaultColor {
		t.Fatalf("accent = %#v, want %#v", got, DefaultColor)
	}
	if got := palette.Background(); got != DefaultColor {
		t.Fatalf("background = %#v, want %#v", got, DefaultColor)
	}
}

func TestPaletteColorAtWraps(t *testing.T) {
	palette := testPalette()

	tests := map[string]struct {
		index int
		want  Color
	}{
		"first base": {
			index: 0,
			want:  color(10),
		},
		"second base": {
			index: 1,
			want:  color(20),
		},
		"first accent": {
			index: 2,
			want:  color(90),
		},
		"wraps forward": {
			index: 3,
			want:  color(10),
		},
		"wraps backward": {
			index: -1,
			want:  color(90),
		},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			if got := palette.ColorAt(tt.index); got != tt.want {
				t.Fatalf("ColorAt(%d) = %#v, want %#v", tt.index, got, tt.want)
			}
		})
	}
}

func TestPaletteAccentAtWraps(t *testing.T) {
	palette := Palette{Base: []Color{color(10)}, Accents: []Color{color(90), color(120)}}

	if got := palette.AccentAt(3); got != color(120) {
		t.Fatalf("AccentAt(3) = %#v, want %#v", got, color(120))
	}
	if got := palette.AccentAt(-1); got != color(120) {
		t.Fatalf("AccentAt(-1) = %#v, want %#v", got, color(120))
	}
}

func TestPaletteGradientStops(t *testing.T) {
	palette := testPalette()

	got := palette.GradientStops(5)
	want := []Color{color(200), color(10), color(20), color(90), color(200)}

	if !reflect.DeepEqual(got, want) {
		t.Fatalf("GradientStops = %#v, want %#v", got, want)
	}
}

func TestPaletteGradientStopsFallback(t *testing.T) {
	got := (Palette{}).GradientStops(2)
	want := []Color{DefaultColor, DefaultColor}

	if !reflect.DeepEqual(got, want) {
		t.Fatalf("GradientStops = %#v, want %#v", got, want)
	}
	if got := (Palette{}).GradientStops(0); got != nil {
		t.Fatalf("GradientStops(0) = %#v, want nil", got)
	}
}

func TestWithBrightnessClampsPercent(t *testing.T) {
	base := color(10)

	tests := map[string]struct {
		value float64
		want  float64
	}{
		"negative": {
			value: -1,
			want:  0,
		},
		"in range": {
			value: 42,
			want:  42,
		},
		"above range": {
			value: 101,
			want:  100,
		},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			if got := WithBrightness(base, tt.value).Brightness; got != tt.want {
				t.Fatalf("brightness = %f, want %f", got, tt.want)
			}
		})
	}
}

func testPalette() Palette {
	return Palette{
		Name:        "test",
		Base:        []Color{color(10), color(20)},
		Accents:     []Color{color(90)},
		Backgrounds: []Color{color(200)},
	}
}

func color(hue float64) Color {
	return Color{Hue: hue, Saturation: 100, Brightness: 100, Kelvin: 3500}
}
