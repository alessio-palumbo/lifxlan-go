package httpapi

import (
	"bytes"
	"encoding/json"
	"net"
	"net/http"
	"testing"
	"time"

	"github.com/alessio-palumbo/lifxlan-go/pkg/controller"
	"github.com/alessio-palumbo/lifxlan-go/pkg/device"
)

var benchmarkPayloadSink []byte

func BenchmarkDeviceEventSSE(b *testing.B) {
	benchmarks := map[string]struct {
		device  device.Device
		changes controller.DeviceChange
	}{
		"light": {device: device.Device{
			Address:      &net.UDPAddr{IP: net.IPv4(192, 168, 1, 20), Port: 56700},
			Serial:       device.Serial{0xd0, 0x73, 0xd5, 1, 2, 3},
			Label:        "Living Room",
			RegistryName: "LIFX benchmark fixture",
			LocationID:   device.LocationID{1},
			Location:     "Home",
			GroupID:      device.GroupID{2},
			Group:        "Downstairs",
			Type:         device.DeviceTypeLight,
			LightType:    device.LightTypeSingleZone,
			PoweredOn:    true,
			Color:        device.Color{Hue: 210, Saturation: 80, Brightness: 60, Kelvin: 3500},
		}, changes: controller.DeviceChangeLight},
		"switch_4": {device: device.Device{
			Serial: device.Serial{0xd0, 0x73, 0xd5, 4, 5, 6},
			Label:  "Wall Switch",
			Type:   device.DeviceTypeSwitch,
			Relays: []device.Relay{{Index: 0, PoweredOn: true}, {Index: 1}, {Index: 2, PoweredOn: true}, {Index: 3}},
		}, changes: controller.DeviceChangeRelays},
	}

	for name, fixture := range benchmarks {
		b.Run(name, func(b *testing.B) {
			event := controller.DeviceEvent{
				Type:     controller.DeviceEventUpdated,
				Device:   fixture.device,
				Changes:  fixture.changes,
				Revision: 42,
			}
			writer := &benchmarkSSEWriter{header: make(http.Header)}
			responseController := http.NewResponseController(writer)
			var frameSize int

			b.ReportAllocs()
			b.ResetTimer()
			for b.Loop() {
				payload, err := json.Marshal(newDeviceEventResponse(event))
				if err != nil {
					b.Fatal(err)
				}
				if err := writeSSE(responseController, writer, event.Revision, event.Type.String(), payload); err != nil {
					b.Fatal(err)
				}
				frameSize = writer.Len()
				writer.Reset()
				benchmarkPayloadSink = payload
			}
			b.ReportMetric(float64(frameSize), "wire-B/event")
		})
	}
}

type benchmarkSSEWriter struct {
	bytes.Buffer
	header http.Header
}

func (w *benchmarkSSEWriter) Header() http.Header              { return w.header }
func (w *benchmarkSSEWriter) WriteHeader(int)                  {}
func (w *benchmarkSSEWriter) Flush()                           {}
func (w *benchmarkSSEWriter) SetWriteDeadline(time.Time) error { return nil }
