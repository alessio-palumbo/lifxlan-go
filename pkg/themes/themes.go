// Package themes builds deterministic static color plans without network I/O.
package themes

import (
	"fmt"
	"math"
	"slices"
	"strings"
	"time"

	"github.com/alessio-palumbo/lifxlan-go/pkg/device"
	"github.com/alessio-palumbo/lifxlan-go/pkg/effects"
)

// Layout controls how palette colors cover a multizone or matrix surface.
type Layout string

const (
	// Gradient smoothly interpolates palette stops across the selected axis.
	Gradient Layout = "gradient"
	// Steps divides the selected axis into contiguous palette-color bands.
	Steps Layout = "steps"
	// Solid fills each target with its assigned palette color.
	Solid Layout = "solid"
)

// Axis chooses the direction of gradient and stepped layouts.
type Axis string

const (
	Horizontal Axis = "horizontal"
	Vertical   Axis = "vertical"
)

// Theme is a serializable static palette and layout. It contains no targets or
// power changes. Stops are base colors, then accents, then backgrounds.
type Theme struct {
	Name    string          `json:"name"`
	Palette effects.Palette `json:"palette"`
	Layout  Layout          `json:"layout,omitempty"`
	Axis    Axis            `json:"axis,omitempty"`
}

// Application is one target-bound logical frame. Render it through the existing
// effects adapters; those handle physical matrix orientation and packetization.
type Application struct {
	Serial device.Serial
	Frame  effects.Frame
}

// Validate checks a theme before discovery or network writes.
func (t Theme) Validate() error {
	if strings.TrimSpace(t.Name) == "" {
		return fmt.Errorf("theme name is required")
	}
	switch t.Layout {
	case "", Gradient, Steps, Solid:
	default:
		return fmt.Errorf("unknown theme layout %q", t.Layout)
	}
	switch t.Axis {
	case "", Horizontal, Vertical:
	default:
		return fmt.Errorf("unknown theme axis %q", t.Axis)
	}
	colors := t.colors()
	if len(colors) == 0 {
		return fmt.Errorf("theme palette must contain at least one color")
	}
	for i, c := range colors {
		if !inRange(c.Hue, 0, 360) || !inRange(c.Saturation, 0, 100) || !inRange(c.Brightness, 0, 100) || c.Kelvin < 1500 || c.Kelvin > 9000 {
			return fmt.Errorf("invalid palette color %d: require hue 0..360, saturation/brightness 0..100, kelvin 1500..9000", i)
		}
	}
	return nil
}

// Plan validates all targets and returns independent frames in serial order.
// Single-zone lights receive successive palette colors, wrapping when needed.
// It trusts the caller's geometry; controller consumers should obtain complete
// observed state before planning. Plan never changes power or sends messages.
func (t Theme) Plan(devices []device.Device, duration time.Duration) ([]Application, error) {
	if err := t.Validate(); err != nil {
		return nil, err
	}
	if duration < 0 || duration > time.Duration(math.MaxUint32)*time.Millisecond {
		return nil, fmt.Errorf("theme duration must fit a nonnegative uint32 millisecond transition")
	}
	if len(devices) == 0 {
		return nil, fmt.Errorf("theme requires at least one light")
	}
	sorted := slices.Clone(devices)
	slices.SortFunc(sorted, func(a, b device.Device) int { return strings.Compare(a.Serial.String(), b.Serial.String()) })
	colors := t.colors()
	plan := make([]Application, 0, len(sorted))
	singleIndex := 0
	var previous device.Serial
	for i, d := range sorted {
		if d.Serial == (device.Serial{}) {
			return nil, fmt.Errorf("theme target serial is missing")
		}
		if d.Type != device.DeviceTypeLight && d.Type != device.DeviceTypeHybrid {
			return nil, fmt.Errorf("%s: theme target must be a light", d.Serial)
		}
		if d.ProductID == 0 {
			return nil, fmt.Errorf("%s: product information is missing", d.Serial)
		}
		switch d.LightType {
		case device.LightTypeSingleZone, device.LightTypeMultiZone, device.LightTypeMatrix:
		default:
			return nil, fmt.Errorf("%s: unsupported light type", d.Serial)
		}
		if i > 0 && d.Serial == previous {
			return nil, fmt.Errorf("duplicate theme target %s", d.Serial)
		}
		previous = d.Serial
		caps := effects.CapabilitiesFromDevice(d)
		if caps.Width <= 0 || caps.Height <= 0 || caps.Width > math.MaxInt/caps.Height {
			return nil, fmt.Errorf("%s: surface geometry is missing or invalid", d.Serial)
		}
		if d.LightType == device.LightTypeMatrix && (d.MatrixProperties.Width <= 0 || d.MatrixProperties.Height <= 0 || d.MatrixProperties.ChainLength <= 0) {
			return nil, fmt.Errorf("%s: matrix geometry is missing", d.Serial)
		}
		assigned := i
		if d.LightType == device.LightTypeSingleZone {
			assigned = singleIndex
			singleIndex++
		}
		frame := effects.NewFrame(caps.Width, caps.Height, duration, colors[assigned%len(colors)])
		if d.LightType != device.LightTypeSingleZone && t.Layout != Solid {
			span := caps.Width
			if t.Axis == Vertical {
				span = caps.Height
			}
			for y := 0; y < caps.Height; y++ {
				for x := 0; x < caps.Width; x++ {
					position := x
					if t.Axis == Vertical {
						position = y
					}
					color := colors[0]
					if t.Layout == Steps {
						color = colors[position*len(colors)/span]
					} else if span > 1 {
						color = sampleGradient(colors, float64(position)/float64(span-1))
					}
					frame.Colors[y*caps.Width+x] = color
				}
			}
		}
		for j, c := range frame.Colors {
			if !d.ColorProperties.HasColor {
				c.Saturation = 0
			}
			rangeK := d.ColorProperties.TemperatureRange
			if rangeK.Min > 0 && rangeK.Max >= rangeK.Min {
				c.Kelvin = uint16(max(rangeK.Min, min(rangeK.Max, int(c.Kelvin))))
			}
			frame.Colors[j] = c
		}
		plan = append(plan, Application{Serial: d.Serial, Frame: frame})
	}
	return plan, nil
}

func (t Theme) colors() []effects.Color {
	colors := append([]effects.Color{}, t.Palette.Base...)
	colors = append(colors, t.Palette.Accents...)
	return append(colors, t.Palette.Backgrounds...)
}

func inRange(value, min, max float64) bool {
	return !math.IsNaN(value) && !math.IsInf(value, 0) && value >= min && value <= max
}

func sampleGradient(colors []effects.Color, position float64) effects.Color {
	if len(colors) == 1 {
		return colors[0]
	}
	index := position * float64(len(colors)-1)
	i := int(index)
	if i >= len(colors)-1 {
		return colors[len(colors)-1]
	}
	a, b := colors[i], colors[i+1]
	t := index - float64(i)
	delta := math.Mod(b.Hue-a.Hue+540, 360) - 180
	hue := math.Mod(a.Hue+delta*t+360, 360)
	return effects.Color{Hue: hue, Saturation: a.Saturation + (b.Saturation-a.Saturation)*t, Brightness: a.Brightness + (b.Brightness-a.Brightness)*t, Kelvin: uint16(math.Round(float64(a.Kelvin) + (float64(b.Kelvin)-float64(a.Kelvin))*t))}
}
