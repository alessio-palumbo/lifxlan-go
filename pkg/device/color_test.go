package device

import (
	"math"
	"testing"

	"github.com/alessio-palumbo/lifxprotocol-go/gen/protocol/packets"
	"github.com/stretchr/testify/assert"
)

func TestNewColor(t *testing.T) {
	tests := []struct {
		hsbk      packets.LightHsbk
		wantColor Color
	}{
		{
			packets.LightHsbk{},
			Color{},
		},
		{
			packets.LightHsbk{Hue: 16384, Saturation: 16384, Brightness: 16384, Kelvin: 3500},
			Color{Hue: 90, Saturation: 25, Brightness: 25, Kelvin: 3500},
		},
		{
			packets.LightHsbk{Hue: 32768, Saturation: 32768, Brightness: 32768, Kelvin: 3500},
			Color{Hue: 180, Saturation: 50, Brightness: 50, Kelvin: 3500},
		},
		{
			packets.LightHsbk{Hue: 49151, Saturation: 49151, Brightness: 49151, Kelvin: 3500},
			Color{Hue: 270, Saturation: 75, Brightness: 75, Kelvin: 3500},
		},
		{
			packets.LightHsbk{Hue: math.MaxUint16, Saturation: math.MaxUint16, Brightness: math.MaxUint16, Kelvin: 3500},
			Color{Hue: 360, Saturation: 100, Brightness: 100, Kelvin: 3500},
		},
	}

	for _, tt := range tests {
		assert.Equal(t, tt.wantColor, NewColor(tt.hsbk))
	}
}

func TestToDeviceColor(t *testing.T) {
	tests := []struct {
		color    Color
		wantHsbk packets.LightHsbk
	}{
		{
			Color{},
			packets.LightHsbk{},
		},
		{
			Color{Hue: 90, Saturation: 25, Brightness: 25, Kelvin: 3500},
			packets.LightHsbk{Hue: 16384, Saturation: 16384, Brightness: 16384, Kelvin: 3500},
		},
		{
			Color{Hue: 180, Saturation: 50, Brightness: 50, Kelvin: 3500},
			packets.LightHsbk{Hue: 32768, Saturation: 32768, Brightness: 32768, Kelvin: 3500},
		},
		{
			Color{Hue: 270, Saturation: 75, Brightness: 75, Kelvin: 3500},
			packets.LightHsbk{Hue: 49151, Saturation: 49151, Brightness: 49151, Kelvin: 3500},
		},
		{
			Color{Hue: 360, Saturation: 100, Brightness: 100, Kelvin: 3500},
			packets.LightHsbk{Hue: math.MaxUint16, Saturation: math.MaxUint16, Brightness: math.MaxUint16, Kelvin: 3500},
		},
	}

	for _, tt := range tests {
		assert.Equal(t, tt.wantHsbk, tt.color.ToDeviceColor())
	}
}

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
		c := &Color{Kelvin: uint16(tt.kelvin)}
		r, g, b := c.KelvinToRGB()
		if r != tt.wantR || g != tt.wantG || b != tt.wantB {
			t.Errorf("KelvinToRGB(%d) = (%d,%d,%d), want (%d,%d,%d)", tt.kelvin, r, g, b, tt.wantR, tt.wantG, tt.wantB)
		}
	}
}
