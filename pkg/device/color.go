package device

import (
	"fmt"
	"math"

	"github.com/alessio-palumbo/lifxprotocol-go/gen/protocol/packets"
)

// Color represent a HSBK Color.
type Color struct {
	Hue        float64
	Saturation float64
	Brightness float64
	Kelvin     uint16
}

// NewColor parse a lifxprotocol-go LightHsbk into a Color
// converting device used values into human readable ones.
func NewColor(hsbk packets.LightHsbk) Color {
	return Color{
		Hue:        ConvertDeviceValueToExternal(hsbk.Hue, 360),
		Saturation: ConvertDeviceValueToExternal(hsbk.Saturation, 100),
		Brightness: ConvertDeviceValueToExternal(hsbk.Brightness, 100),
		Kelvin:     hsbk.Kelvin,
	}
}

// ToDeviceColor converts a Color to lifxprotocol-go LightHsbk.
func (c Color) ToDeviceColor() packets.LightHsbk {
	return packets.LightHsbk{
		Hue:        ConvertExternalToDeviceValue(c.Hue, 360),
		Saturation: ConvertExternalToDeviceValue(c.Saturation, 100),
		Brightness: ConvertExternalToDeviceValue(c.Brightness, 100),
		Kelvin:     c.Kelvin,
	}
}

// String converts a Color into string for easy logging.
func (c Color) String() string {
	if c.Saturation == 0 {
		return fmt.Sprintf("Brightness: %f%% Kelvin: %d", c.Brightness, c.Kelvin)
	}
	return fmt.Sprintf("Brightness: %f%%, Hue: %f, Saturation: %f%%", c.Brightness, c.Hue, c.Saturation)
}

// HSBToRGB converts the color from Hue, Saturation, Brightness (HSB) format
// to Red, Green, Blue (RGB) format. Hue is expected in degrees [0,360),
// Saturation and Brightness are percentages [0,100]. The resulting RGB
// components are returned as integers in the range [0,255].
func (c *Color) HSBToRGB() (int, int, int) {
	h, s, b := c.Hue, c.Saturation, c.Brightness

	s, b = s/100, b/100
	if s == 0.0 {
		return int(b * 255), int(b * 255), int(b * 255)
	}

	h = math.Mod(h, 360)
	hi := math.Floor(h / 60)
	f := h/60 - hi
	p := b * (1 - s)
	q := b * (1 - f*s)
	t := b * (1 - (1-f)*s)

	switch int(hi) {
	case 0:
		return int(b * 255), int(t * 255), int(p * 255)
	case 1:
		return int(q * 255), int(b * 255), int(p * 255)
	case 2:
		return int(p * 255), int(b * 255), int(t * 255)
	case 3:
		return int(p * 255), int(q * 255), int(b * 255)
	case 4:
		return int(t * 255), int(p * 255), int(b * 255)
	case 5:
		return int(b * 255), int(p * 255), int(q * 255)
	}

	return 0, 0, 0
}

// KelvinToRGB converts a color temperature in Kelvin to an RGB color.
// It uses a standard approximation suitable for many applications,
// but accuracy is best between 1000K and 40000K.
func (c *Color) KelvinToRGB() (r, g, b int) {
	temp := int(math.Round(float64(c.Kelvin) / 100.0))

	// Red
	if temp <= 66 {
		r = 255
	} else {
		r = temp - 60
		r = int(329.698727446 * math.Pow(float64(r), -0.1332047592))
		r = min(max(r, 0), 255)
	}

	// Green
	if temp <= 66 {
		g = temp
		g = int(99.4708025861*math.Log(float64(g)) - 161.1195681661)
		g = min(max(g, 0), 255)
	} else {
		g = temp - 60
		g = int(288.1221695283 * math.Pow(float64(g), -0.0755148492))
		g = min(max(g, 0), 255)
	}

	// Blue
	if temp >= 66 {
		b = 255
	} else if temp <= 19 {
		b = 0
	} else {
		b = temp - 10
		b = int(138.5177312231*math.Log(float64(b)) - 305.0447927307)
		b = min(max(b, 0), 255)
	}

	return int(r), int(g), int(b)
}

// ConvertDeviceValueToExternal takes a device value in the range 0-65535
// and converts it into the range defined by the multiplier.
func ConvertDeviceValueToExternal(v uint16, multiplier float64) float64 {
	return math.Round(float64(v) / math.MaxUint16 * multiplier)
}

// ConvertExternalToDeviceValue takes an external value and multiplier
// and converts it into a device value 0-65535.
func ConvertExternalToDeviceValue(v float64, multiplier float64) uint16 {
	return uint16(math.Round(v * math.MaxUint16 / multiplier))
}
