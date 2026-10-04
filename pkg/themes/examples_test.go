package themes_test

import (
	"fmt"
	"time"

	"github.com/alessio-palumbo/lifxlan-go/pkg/device"
	"github.com/alessio-palumbo/lifxlan-go/pkg/effects"
	"github.com/alessio-palumbo/lifxlan-go/pkg/themes"
)

func ExampleTheme_PlanWithOptions() {
	// Live clients must obtain complete observed state before creating frames.
	d := device.Device{
		Serial: device.Serial{0xd0, 0x73, 0xd5, 0, 0, 1}, ProductID: 27,
		Type: device.DeviceTypeLight, LightType: device.LightTypeSingleZone,
		ColorProperties: device.ColorProperties{HasColor: true},
		Color:           effects.Color{Hue: 120, Saturation: 80, Brightness: 35, Kelvin: 3500},
	}
	initial, err := effects.FrameFromDeviceState(d, 0)
	if err != nil {
		panic(err)
	}
	theme := themes.Theme{Name: "Warm", Palette: effects.Palette{Base: []effects.Color{
		{Hue: 30, Saturation: 70, Brightness: 80, Kelvin: 4000},
	}}}
	plan, err := theme.PlanWithOptions([]device.Device{d}, time.Second, themes.PlanOptions{
		Brightness:    themes.PreserveBrightness,
		InitialFrames: map[device.Serial]effects.Frame{d.Serial: initial},
	})
	if err != nil {
		panic(err)
	}
	c := plan[0].Frame.Colors[0]
	fmt.Println(c.Hue, c.Saturation, c.Brightness, c.Kelvin)
	// Output:
	// 30 70 35 4000
}
