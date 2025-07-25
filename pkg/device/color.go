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
		Hue:        convertDeviceValueToExternal(hsbk.Hue, 360),
		Saturation: convertDeviceValueToExternal(hsbk.Saturation, 100),
		Brightness: convertDeviceValueToExternal(hsbk.Brightness, 100),
		Kelvin:     hsbk.Kelvin,
	}
}

// String converts a Color into string for easy logging.
func (c *Color) String() string {
	if c.Saturation == 0 {
		return fmt.Sprintf("Brightness: %f%% Kelvin: %d", c.Brightness, c.Kelvin)
	}
	return fmt.Sprintf("Brightness: %f%%, Hue: %f, Saturation: %f%%", c.Brightness, c.Hue, c.Saturation)
}

// convertDeviceValueToExternal takes a device value in the range 0-65535
// and converts it into the range defined by the multiplier.
func convertDeviceValueToExternal(v uint16, multiplier float64) float64 {
	return math.Round(float64(v) / math.MaxUint16 * multiplier)
}
