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

// BlankColor returns the standard off color used for blank frame cells.
func BlankColor() Color {
	return blankColor
}

// FrameSize returns the number of colors needed for a width by height frame.
func FrameSize(width, height int) int {
	if width <= 0 || height <= 0 {
		return 0
	}
	return width * height
}

// NewFrame returns a logical frame filled with color.
func NewFrame(width, height int, duration time.Duration, color Color) Frame {
	colors := make([]Color, FrameSize(width, height))
	for i := range colors {
		colors[i] = color
	}
	return Frame{
		Colors:   colors,
		Width:    width,
		Height:   height,
		Duration: duration,
	}
}

// ValidateFrame validates frame dimensions and color count.
func ValidateFrame(frame Frame) error {
	return validateFrame(frame)
}

// FrameColor returns the color at x,y and whether the coordinate is valid.
func FrameColor(frame Frame, x, y int) (Color, bool) {
	index, ok := frameIndex(frame, x, y)
	if !ok {
		return Color{}, false
	}
	return frame.Colors[index], true
}

// SetFrameColor sets the color at x,y and reports whether the coordinate is valid.
func SetFrameColor(frame *Frame, x, y int, color Color) bool {
	if frame == nil {
		return false
	}
	index, ok := frameIndex(*frame, x, y)
	if !ok {
		return false
	}
	frame.Colors[index] = color
	return true
}

func frameIndex(frame Frame, x, y int) (int, bool) {
	if frame.Width <= 0 || frame.Height <= 0 || x < 0 || y < 0 || x >= frame.Width || y >= frame.Height {
		return 0, false
	}
	index := y*frame.Width + x
	if index < 0 || index >= len(frame.Colors) {
		return 0, false
	}
	return index, true
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
		surface := device.SurfaceFromDevice(d)
		c.Width = surface.Width
		c.Height = surface.Height
		c.Zones = surface.Zones
		if surface.Matrix != nil {
			c.ChainLength = len(surface.Matrix.Chains)
		}
		c.ChainOrientations = slices.Clone(d.MatrixProperties.ChainOrientations)
	default:
		c.Zones = 1
		c.Width = 1
		c.Height = 1
	}

	return c
}
