package client

import (
	"fmt"
	"math"

	"github.com/alessio-palumbo/lifxprotocol-go/gen/protocol/packets"
)

type Color struct {
	Hue        float64
	Saturation float64
	Brightness float64
	Kelvin     uint16
}

func NewColor(hsbk packets.LightHsbk) Color {
	return Color{
		Hue:        convertDeviceValueToExternal(hsbk.Hue, 360),
		Saturation: convertDeviceValueToExternal(hsbk.Saturation, 100),
		Brightness: convertDeviceValueToExternal(hsbk.Brightness, 100),
		Kelvin:     hsbk.Kelvin,
	}
}

func (c *Color) String() string {
	if c.Saturation == 0 {
		return fmt.Sprintf("Brightness: %f%% Kelvin: %d", c.Brightness, c.Kelvin)
	}
	return fmt.Sprintf("Brightness: %f%%, Hue: %f, Saturation: %f%%", c.Brightness, c.Hue, c.Saturation)
}

func convertDeviceValueToExternal(v uint16, multiplier float64) float64 {
	return math.Round(float64(v) / math.MaxUint16 * multiplier)
}

func convertExternalToDeviceValue(v float64, multiplier float64) uint16 {
	return uint16(math.Round(v * math.MaxUint16 / multiplier))
}
