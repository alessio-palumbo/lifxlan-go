package device

import (
	"testing"
)

func TestHSBToRGB(t *testing.T) {
	tests := []struct {
		h, s, b float64
		wantR   int
		wantG   int
		wantB   int
	}{
		{0, 0, 0, 0, 0, 0},          // black
		{0, 0, 100, 255, 255, 255},  // white
		{0, 100, 100, 255, 0, 0},    // red
		{120, 100, 100, 0, 255, 0},  // green
		{240, 100, 100, 0, 0, 255},  // blue
		{60, 100, 100, 255, 255, 0}, // yellow
		{180, 100, 50, 0, 127, 127}, // cyan at 50% brightness
		{300, 50, 50, 127, 63, 127}, // magenta-ish at 50% brightness and 50% saturation
	}

	for _, tt := range tests {
		c := &Color{Hue: tt.h, Saturation: tt.s, Brightness: tt.b}
		r, g, b := c.HSBToRGB()
		if r != tt.wantR || g != tt.wantG || b != tt.wantB {
			t.Errorf("HSBToRGB(%v, %v, %v) = (%d,%d,%d), want (%d,%d,%d)", tt.h, tt.s, tt.b, r, g, b, tt.wantR, tt.wantG, tt.wantB)
		}
	}
}

func TestKelvinToRGB(t *testing.T) {
	tests := []struct {
		kelvin int
		wantR  int
		wantG  int
		wantB  int
	}{
		{1500, 255, 108, 0},   // very warm
		{2000, 255, 136, 13},  // warm
		{3000, 255, 177, 109}, // soft white
		{4000, 255, 205, 166}, // cool white
		{5000, 255, 228, 205}, // daylight
		{6500, 255, 254, 250}, // daylight-ish
		{9000, 209, 222, 255}, // very cool
	}

	for _, tt := range tests {
		c := &Color{}
		r, g, b := c.KelvinToRGB(tt.kelvin)
		if r != tt.wantR || g != tt.wantG || b != tt.wantB {
			t.Errorf("KelvinToRGB(%d) = (%d,%d,%d), want (%d,%d,%d)", tt.kelvin, r, g, b, tt.wantR, tt.wantG, tt.wantB)
		}
	}
}
