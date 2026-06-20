package adapters

import (
	"context"
	"fmt"

	"github.com/alessio-palumbo/lifxlan-go/pkg/device"
	"github.com/alessio-palumbo/lifxlan-go/pkg/effects"
	"github.com/alessio-palumbo/lifxlan-go/pkg/protocol"
	"github.com/alessio-palumbo/lifxprotocol-go/gen/protocol/packets"
)

func ExampleNewRendererForDevice() {
	dev := device.Device{
		LightType: device.LightTypeMultiZone,
		MultizoneProperties: device.MultizoneProperties{
			Zones: make([]packets.LightHsbk, 2),
		},
	}
	send := func(msg *protocol.Message) error {
		fmt.Printf("%T\n", msg.Payload)
		return nil
	}

	effect := effects.NewSolid(effects.SolidConfig{
		Capabilities: effects.CapabilitiesFromDevice(dev),
		Color:        effects.Color{Hue: 200, Saturation: 100, Brightness: 30, Kelvin: 3500},
	})
	frame, _ := effect.Next(0)

	renderer := NewRendererForDevice(dev, send)
	_ = renderer.RenderFrame(context.Background(), frame)

	// Output:
	// *packets.MultiZoneExtendedSetColorZones
}
