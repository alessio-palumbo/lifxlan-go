package effects

import (
	"fmt"
	"time"

	"github.com/alessio-palumbo/lifxlan-go/pkg/device"
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
