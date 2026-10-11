package themes

import (
	"context"
	"fmt"
	"time"

	"github.com/alessio-palumbo/lifxlan-go/pkg/device"
	"github.com/alessio-palumbo/lifxlan-go/pkg/effects"
	"github.com/alessio-palumbo/lifxlan-go/pkg/effects/adapters"
	"github.com/alessio-palumbo/lifxlan-go/pkg/messages"
	"github.com/alessio-palumbo/lifxlan-go/pkg/protocol"
)

// PartPlanOptions selects parts and deterministic variation at application time,
// never in the saved theme. Empty Part means all available parts. Empty Brightness
// means palette brightness. Initial brightness comes from the supplied observed
// devices, not a separate InitialFrames map.
type PartPlanOptions struct {
	Part         device.LightPart
	Variation    uint64
	Reverse      bool
	Seed         uint64
	MatrixLayout MatrixLayout
	Brightness   BrightnessPolicy
}

// PartApplication contains preview colors and the exact composed control packets.
// Frame previews main (unused for uplight-only selection); Uplight is nil when
// unselected or unsupported. Send Messages directly: rendering Frame alone loses
// the part composition. Messages and frames do not alias input state. A successful
// send is not an acknowledgement; multi-device application is not atomic.
type PartApplication struct {
	Application
	Part     device.LightPart
	Uplight  *effects.Color
	Messages []*protocol.Message
}

// PlanParts plans colors and packets without network calls, power changes or
// effect stopping. Devices must come from complete observed state whenever parts
// are preserved or brightness is retained. Buffer lengths validate geometry only;
// the pure planner cannot distinguish unobserved allocations from valid black.
// Legacy Plan/PlanWithOptions and frame renderers remain unchanged.
func (t Theme) PlanParts(devices []device.Device, duration time.Duration, opts PartPlanOptions) ([]PartApplication, error) {
	part := opts.Part
	if part == "" {
		part = device.LightPartAll
	}
	if part != device.LightPartAll && part != device.LightPartMain && part != device.LightPartUplight {
		return nil, fmt.Errorf("unknown light part %q", part)
	}
	base := PlanOptions{Variation: opts.Variation, Reverse: opts.Reverse, Seed: opts.Seed, MatrixLayout: opts.MatrixLayout, Brightness: opts.Brightness}
	// Preflight bounds and target validation before allocating initial frames or
	// composing physical buffers. Legacy planning keeps assignments unchanged.
	preflight := base
	preflight.Brightness = PaletteBrightness
	if opts.Brightness != "" && opts.Brightness != PaletteBrightness && opts.Brightness != PreserveBrightness {
		return nil, fmt.Errorf("unknown brightness policy %q", opts.Brightness)
	}
	plan, err := t.PlanWithOptions(devices, duration, preflight)
	if err != nil {
		return nil, err
	}
	bySerial := make(map[device.Serial]device.Device, len(devices))
	for _, d := range devices {
		if part == device.LightPartUplight && !device.HasUplight(d) {
			return nil, fmt.Errorf("%s: uplight unsupported", d.Serial)
		}
		bySerial[d.Serial] = d
	}
	if opts.Brightness == PreserveBrightness {
		base.InitialFrames = make(map[device.Serial]effects.Frame, len(devices))
		for _, d := range devices {
			if err := completePartState(d); err != nil {
				return nil, fmt.Errorf("%s: %w", d.Serial, err)
			}
			var f effects.Frame
			var err error
			if d.LightType != device.LightTypeSingleZone {
				// Use the exact physical adapter to avoid quantizing per-cell
				// brightness through human-readable cached percentages.
				f, err = effects.AdaptPhysicalColorStateToFrame(effects.PhysicalColorState{Zones: d.MultizoneProperties.Zones, MatrixChains: d.MatrixProperties.ChainZones}, device.SurfaceFromDevice(d), 0)
			} else {
				f, err = effects.FrameFromDeviceState(d, 0)
			}
			if err != nil {
				return nil, err
			}
			base.InitialFrames[d.Serial] = f
		}
		plan, err = t.PlanWithOptions(devices, duration, base)
		if err != nil {
			return nil, err
		}
	}
	colors := variationColors(t.colors(), opts.Variation, opts.Reverse)
	out := make([]PartApplication, 0, len(plan))
	for i, application := range plan {
		d := bySerial[application.Serial]
		result := PartApplication{Application: application, Part: part}
		if !device.HasUplight(d) {
			r := adapters.NewRendererForDevice(d, func(msg *protocol.Message) error { result.Messages = append(result.Messages, msg); return nil })
			if err := r.RenderFrame(context.Background(), application.Frame); err != nil {
				return nil, fmt.Errorf("%s: %w", d.Serial, err)
			}
		} else {
			var main *effects.Frame
			if part != device.LightPartUplight {
				main = &application.Frame
			}
			if part != device.LightPartMain {
				// Stable sorted-target assignment, with the same variation/reversal
				// palette as main. Changing the target set can change assignment.
				color := colors[i%len(colors)]
				if !d.ColorProperties.HasColor {
					color.Saturation = 0
				}
				k := d.ColorProperties.TemperatureRange
				if k.Min > 0 && k.Max >= k.Min {
					color.Kelvin = uint16(max(k.Min, min(k.Max, int(color.Kelvin))))
				}
				result.Uplight = &color
			}
			physical, err := effects.ComposeLightPartFrame(d, main, result.Uplight, opts.Brightness == PreserveBrightness)
			if err != nil {
				return nil, fmt.Errorf("%s: %w", d.Serial, err)
			}
			if result.Uplight != nil && opts.Brightness == PreserveBrightness {
				mask, err := effects.LightPartPhysicalMask(d, device.LightPartUplight)
				if err != nil {
					return nil, err
				}
				for j, c := range mask {
					if c.Brightness != 0 {
						result.Uplight.Brightness = float64(physical[j].Brightness) * 100 / 65535
						break
					}
				}
			}
			result.Messages = messages.SetMatrixColorsFromSlice(0, 1, d.MatrixProperties.Width, physical, duration)
		}
		out = append(out, result)
	}
	return out, nil
}

func completePartState(d device.Device) error {
	switch d.LightType {
	case device.LightTypeMatrix:
		m := d.MatrixProperties
		if m.ChainLength <= 0 || len(m.ChainZones) != m.ChainLength {
			return fmt.Errorf("incomplete initial matrix state")
		}
		for _, chain := range m.ChainZones {
			if len(chain) != m.Width*m.Height {
				return fmt.Errorf("incompatible initial matrix state")
			}
		}
	case device.LightTypeMultiZone:
		if len(d.MultizoneProperties.Zones) == 0 {
			return fmt.Errorf("missing initial multizone state")
		}
	}
	return nil
}
