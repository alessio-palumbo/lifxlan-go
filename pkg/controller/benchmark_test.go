package controller

import (
	"fmt"
	"net"
	"runtime"
	"testing"

	"github.com/alessio-palumbo/lifxlan-go/pkg/device"
	"github.com/alessio-palumbo/lifxprotocol-go/gen/protocol/packets"
)

var benchmarkEventSink DeviceEvent

func BenchmarkControllerGetDevices(b *testing.B) {
	for _, count := range []int{1, 10, 50, 100} {
		ctrl := benchmarkController(count)
		b.Run(fmt.Sprintf("devices_%d", count), func(b *testing.B) {
			b.ReportAllocs()
			for b.Loop() {
				devices := ctrl.GetDevices()
				runtime.KeepAlive(devices)
			}
		})
	}
}

func BenchmarkControllerGetDevicesParallel(b *testing.B) {
	for _, count := range []int{10, 100} {
		ctrl := benchmarkController(count)
		b.Run(fmt.Sprintf("devices_%d", count), func(b *testing.B) {
			b.ReportAllocs()
			b.RunParallel(func(pb *testing.PB) {
				for pb.Next() {
					devices := ctrl.GetDevices()
					runtime.KeepAlive(devices)
				}
			})
		})
	}
}

func BenchmarkDeviceEventFanout(b *testing.B) {
	fixtures := []struct {
		name    string
		index   int
		changes DeviceChange
	}{
		{name: "single_zone", index: 0, changes: DeviceChangeLight},
		{name: "multizone_82", index: 1, changes: DeviceChangeMultizone},
		{name: "matrix_5x64", index: 2, changes: DeviceChangeMatrix},
	}

	for _, fixture := range fixtures {
		b.Run("device="+fixture.name, func(b *testing.B) {
			for _, subscribers := range []int{0, 1, 5, 10} {
				b.Run(fmt.Sprintf("subscribers=%d", subscribers), func(b *testing.B) {
					ctrl := &Controller{subscriptions: make(map[uint64]*deviceSubscription)}
					for i := range subscribers {
						ctrl.subscriptions[uint64(i+1)] = newDeviceSubscription(1)
					}
					event := DeviceEvent{
						Type:    DeviceEventUpdated,
						Device:  benchmarkControllerDevice(fixture.index),
						Changes: fixture.changes,
					}

					b.ReportAllocs()
					for b.Loop() {
						ctrl.publishDeviceEvent(event)
						for _, subscription := range ctrl.subscriptions {
							benchmarkEventSink, _ = subscription.next()
						}
					}
				})
			}
		})
	}
}

func benchmarkController(count int) *Controller {
	ctrl := &Controller{sessions: make(map[device.Serial]*deviceSession, count)}
	for i := range count {
		d := benchmarkControllerDevice(i)
		ctrl.sessions[d.Serial] = &deviceSession{device: &d}
	}
	return ctrl
}

func benchmarkControllerDevice(index int) device.Device {
	d := device.Device{
		Address:      &net.UDPAddr{IP: net.IPv4(192, 168, byte(index/250), byte(index%250+1)), Port: 56700},
		Serial:       device.Serial{0xd0, 0x73, 0xd5, byte(index >> 16), byte(index >> 8), byte(index)},
		Label:        fmt.Sprintf("Device %03d", index),
		RegistryName: "LIFX benchmark fixture",
		LocationID:   device.LocationID{1},
		Location:     "Home",
		GroupID:      device.GroupID{2},
		Group:        "Benchmark",
		Type:         device.DeviceTypeLight,
		LightType:    device.LightTypeSingleZone,
		PoweredOn:    true,
		Color:        device.Color{Hue: 210, Saturation: 80, Brightness: 60, Kelvin: 3500},
	}

	switch index % 4 {
	case 1:
		d.LightType = device.LightTypeMultiZone
		d.MultizoneProperties.Zones = make([]packets.LightHsbk, 82)
	case 2:
		d.LightType = device.LightTypeMatrix
		d.MatrixProperties = device.MatrixProperties{
			Width:             8,
			Height:            8,
			NZones:            64,
			StatePackets:      1,
			ChainLength:       5,
			ChainZones:        make([][]packets.LightHsbk, 5),
			ChainOrientations: make([]device.Orientation, 5),
		}
		for i := range d.MatrixProperties.ChainZones {
			d.MatrixProperties.ChainZones[i] = make([]packets.LightHsbk, 64)
		}
	case 3:
		d.Type = device.DeviceTypeSwitch
		d.Relays = []device.Relay{
			{Index: 0, PoweredOn: true},
			{Index: 1},
			{Index: 2, PoweredOn: true},
			{Index: 3},
		}
	}

	return d
}
