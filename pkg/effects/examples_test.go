package effects

import (
	"fmt"
	"time"

	"github.com/alessio-palumbo/lifxlan-go/pkg/device"
	"github.com/alessio-palumbo/lifxprotocol-go/gen/protocol/packets"
)

type examplePulse struct {
	caps       Capabilities
	color      Color
	brightness float64
}

func (e *examplePulse) Next(dt time.Duration) (Frame, bool) {
	frame := NewFrame(max(e.caps.Width, 1), max(e.caps.Height, 1), dt, BlankColor())
	color := WithBrightness(e.color, e.brightness)
	_ = SetFrameColor(&frame, 0, 0, color)
	e.brightness += 25
	if e.brightness > 100 {
		e.brightness = 0
	}
	return frame, true
}

func (e *examplePulse) Reset() {
	e.brightness = 25
}

func Example_customEffect() {
	effect := &examplePulse{
		caps:  Capabilities{LightType: device.LightTypeMatrix, Width: 2, Height: 1},
		color: Color{Hue: 30, Saturation: 100, Brightness: 100, Kelvin: 3500},
	}

	frames := Render(effect, 100*time.Millisecond, 300*time.Millisecond)
	fmt.Println(len(frames))
	fmt.Println(frames[0].Frame.Colors[0].Brightness)

	// Output:
	// 3
	// 25
}

func ExampleRender() {
	palette := Palette{
		Base: []Color{
			{Hue: 0, Saturation: 100, Brightness: 50, Kelvin: 3500},
			{Hue: 120, Saturation: 100, Brightness: 50, Kelvin: 3500},
		},
	}
	effect := NewGradient(GradientConfig{
		Capabilities: Capabilities{LightType: device.LightTypeMultiZone, Zones: 2},
		Palette:      palette,
	})

	frames := Render(effect, time.Second, 2*time.Second)
	fmt.Println(len(frames))
	fmt.Println(frames[0].At)
	fmt.Println(frames[0].Frame.Width)

	// Output:
	// 2
	// 0s
	// 2
}

func ExampleAdaptPhysicalColorStateToFrame() {
	surface := device.Surface{
		LightType: device.LightTypeMultiZone,
		Width:     3,
		Height:    1,
		Zones:     3,
	}
	state := NewPhysicalColorState(surface)
	received := []packets.LightHsbk{
		{Hue: 12345, Saturation: 54321, Brightness: 40000, Kelvin: 3500},
		{Hue: 23456, Saturation: 43210, Brightness: 30000, Kelvin: 4000},
	}
	if err := state.MergeZoneColors(1, received); err != nil {
		fmt.Println(err)
		return
	}

	frame, err := AdaptPhysicalColorStateToFrame(state, surface, 0)
	if err != nil {
		fmt.Println(err)
		return
	}
	fmt.Println(frame.Width, frame.Height)
	fmt.Println(frame.Colors[1].ToDeviceColor() == received[0])

	// Output:
	// 3 1
	// true
}

func ExampleRegister() {
	err := Register(EffectDefinition{
		ID:          EffectID("example_pulse"),
		Label:       "Example Pulse",
		Description: "Pulse the first logical cell.",
		DeviceKinds: []device.LightType{device.LightTypeMatrix},
		Params: []ParamDefinition{
			{
				Key:     "color",
				Label:   "Color",
				Kind:    ParamColor,
				Default: Color{Hue: 30, Saturation: 100, Brightness: 100, Kelvin: 3500},
			},
		},
		New: func(config Config, caps Capabilities) (Effect, error) {
			color, err := ColorParam(config.Params, "color")
			if err != nil {
				return nil, err
			}
			return &examplePulse{caps: caps, color: color}, nil
		},
	})
	if err != nil {
		fmt.Println(err)
		return
	}

	effect, err := New(Config{ID: EffectID("example_pulse")}, Capabilities{
		LightType: device.LightTypeMatrix,
		Width:     2,
		Height:    1,
	})
	if err != nil {
		fmt.Println(err)
		return
	}

	frame, ok := effect.Next(time.Second)
	fmt.Println(ok)
	fmt.Println(frame.Width)

	// Output:
	// true
	// 2
}
