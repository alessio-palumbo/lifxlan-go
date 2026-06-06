// Package effects defines deterministic, device-neutral lighting effect types.
package effects

import (
	"slices"
	"time"

	"github.com/alessio-palumbo/lifxlan-go/pkg/device"
)

// Color is the logical HSBK color used by generated frames.
//
// It aliases device.Color so frames can be used with existing lifxlan-go device
// and message APIs without conversion.
type Color = device.Color

// Effect produces deterministic logical frames.
type Effect interface {
	Next(dt time.Duration) (Frame, bool)
	Reset()
}

// Frame is a target-free logical color frame.
type Frame struct {
	Colors   []Color
	Width    int
	Height   int
	Duration time.Duration
}

// FrameAt is a logical frame at a deterministic timeline offset.
type FrameAt struct {
	At    time.Duration
	Frame Frame
}

// Capabilities describes the logical rendering surface available to effects.
type Capabilities struct {
	LightType         device.LightType
	Zones             int
	Width             int
	Height            int
	ChainLength       int
	ChainOrientations []device.Orientation
	HasColor          bool
	TemperatureRange  device.TemperatureRange
}

// CapabilitiesFromDevice derives effect capabilities from an existing device.
func CapabilitiesFromDevice(d device.Device) Capabilities {
	c := Capabilities{
		LightType:        d.LightType,
		HasColor:         d.ColorProperties.HasColor,
		TemperatureRange: d.ColorProperties.TemperatureRange,
	}

	switch d.LightType {
	case device.LightTypeMultiZone:
		c.Zones = len(d.MultizoneProperties.Zones)
		c.Width = c.Zones
		c.Height = 1
	case device.LightTypeMatrix:
		c.Width = d.MatrixProperties.Width
		c.Height = d.MatrixProperties.Height
		c.Zones = d.MatrixProperties.NZones
		if c.Zones == 0 {
			c.Zones = c.Width * c.Height
		}
		c.ChainLength = d.MatrixProperties.ChainLength
		c.ChainOrientations = slices.Clone(d.MatrixProperties.ChainOrientations)
	default:
		c.Zones = 1
		c.Width = 1
		c.Height = 1
	}

	return c
}
