package controller

import (
	"context"
	"errors"
	"math"
	"net"
	"reflect"
	"testing"
	"time"

	"github.com/alessio-palumbo/lifxlan-go/pkg/device"
	"github.com/alessio-palumbo/lifxprotocol-go/gen/protocol/packets"
)

func partDevice(pid uint32, width, height int) device.Device {
	d := device.Device{Serial: snapshotSerial(1), Address: snapshotAddr(1), ProductID: pid, Type: device.DeviceTypeLight, LightType: device.LightTypeMatrix,
		ColorProperties:  device.ColorProperties{HasColor: true, TemperatureRange: device.TemperatureRange{Min: 1500, Max: 9000}},
		MatrixProperties: device.MatrixProperties{Width: width, Height: height, ChainLength: 1, StatePackets: 1 + (width*height-1)/64, ChainZones: [][]packets.LightHsbk{make([]packets.LightHsbk, width*height)}}}
	for i := range d.MatrixProperties.ChainZones[0] {
		d.MatrixProperties.ChainZones[0][i] = packets.LightHsbk{Hue: uint16(1000 + i), Saturation: 45678, Brightness: uint16(10000 + i), Kelvin: 3500}
	}
	return d
}

func TestLightPartMasksPreservePhysicalState(t *testing.T) {
	for _, geometry := range []struct {
		pid        uint32
		w, h, main int
	}{{265, 8, 8, 56}, {201, 8, 16, 120}, {201, 16, 8, 120}} {
		for _, part := range []device.LightPart{device.LightPartMain, device.LightPartUplight} {
			d := partDevice(geometry.pid, geometry.w, geometry.h)
			before := d.Clone()
			brightness := 0.0
			msgs, err := lightPartMessages(d, part, LightStateUpdate{Brightness: &brightness, Duration: time.Second})
			if err != nil {
				t.Fatal(err)
			}
			var colors []packets.LightHsbk
			for _, msg := range msgs {
				if p, ok := msg.Payload.(*packets.TileSet64); ok {
					colors = append(colors, p.Colors[:]...)
				}
			}
			colors = colors[:len(d.MatrixProperties.ChainZones[0])]
			changed := 0
			for i, c := range colors {
				old := before.MatrixProperties.ChainZones[0][i]
				if c.Hue != old.Hue || c.Saturation != old.Saturation || c.Kelvin != old.Kelvin {
					t.Fatal("partial update altered untouched fields")
				}
				if c.Brightness != old.Brightness {
					changed++
				}
			}
			want := geometry.main
			if part == device.LightPartUplight {
				want = 1
			}
			if changed != want || !reflect.DeepEqual(d, before) {
				t.Fatalf("part=%s changed=%d want=%d", part, changed, want)
			}
			last := len(colors) - 1
			if part == device.LightPartUplight && colors[last].Brightness != 0 {
				t.Fatal("uplight did not address last physical cell")
			}
			if part == device.LightPartMain && colors[last] != before.MatrixProperties.ChainZones[0][last] {
				t.Fatal("main overwrote uplight")
			}
		}
	}
}

func TestPartAwareControllerObservationsAndFailures(t *testing.T) {
	d := partDevice(265, 8, 8)
	mock := newMockClient()
	ctrl := newSnapshotController(mock, d)
	s := ctrl.sessions[d.Serial]
	seedSnapshotObservations(s)
	s.observations.observe(s.device, &packets.TileState64{Rect: packets.TileBufferRect{Width: 8}})
	brightness := 50.0
	err := ctrl.SetLightPartColor(context.Background(), d.Serial, device.LightPartUplight, LightStateUpdate{Brightness: &brightness}, SnapshotOptions{})
	if err != nil {
		t.Fatal(err)
	}
	var controls int
	for len(mock.sends) > 0 {
		msg := <-mock.sends
		if p, ok := msg.Payload.(*packets.TileSet64); ok {
			controls++
			if p.Colors[63].Brightness != device.ConvertExternalToDeviceValue(50, 100) || p.Colors[0] != d.MatrixProperties.ChainZones[0][0] {
				t.Fatal(p)
			}
		}
		if msg.Type() == 21 || msg.Type() == 117 {
			t.Fatal("color update changed power")
		}
	}
	if controls != 1 {
		t.Fatal(controls)
	}
	// An allocated but unobserved matrix must never be used for preservation.
	s.observations = snapshotObservations{}
	err = ctrl.SetLightPartColor(context.Background(), d.Serial, device.LightPartMain, LightStateUpdate{Brightness: &brightness}, SnapshotOptions{Timeout: 10 * time.Millisecond, PollInterval: time.Millisecond})
	if err == nil {
		t.Fatal("unobserved state accepted")
	}
	for len(mock.sends) > 0 {
		if msg := <-mock.sends; msg.Type() == 715 || msg.Type() == 716 {
			t.Fatal("timeout sent controls")
		}
	}
	for _, update := range []LightStateUpdate{{Power: new(bool)}, {Brightness: floatPointer(math.NaN())}, {}} {
		if err := ctrl.SetLightPartColor(context.Background(), d.Serial, device.LightPartMain, update, SnapshotOptions{}); !errors.Is(err, ErrInvalidState) {
			t.Fatal(err)
		}
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := ctrl.SetLightPartColor(ctx, d.Serial, device.LightPartAll, LightStateUpdate{Brightness: &brightness}, SnapshotOptions{}); !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
	if err := ctrl.SetLightPartColor(context.Background(), d.Serial, "bad", LightStateUpdate{Brightness: &brightness}, SnapshotOptions{}); !errors.Is(err, ErrInvalidState) {
		t.Fatal(err)
	}
}

func floatPointer(v float64) *float64 { return &v }

func TestPartAllUsesExistingColorSemanticsAndRejectsMissingPart(t *testing.T) {
	d := partDevice(265, 8, 8)
	mock := newMockClient()
	ctrl := newSnapshotController(mock, d)
	brightness := 0.0
	if err := ctrl.SetLightPartColor(context.Background(), d.Serial, device.LightPartAll, LightStateUpdate{Brightness: &brightness}, SnapshotOptions{}); err != nil {
		t.Fatal(err)
	}
	if msg := <-mock.sends; msg.Type() != 119 {
		t.Fatal("all did not reuse optional waveform")
	}
	ctrl.sessions[d.Serial].device.ProductID = 215
	if err := ctrl.SetLightPartColor(context.Background(), d.Serial, device.LightPartUplight, LightStateUpdate{Brightness: &brightness}, SnapshotOptions{}); !errors.Is(err, ErrUnsupportedCapability) {
		t.Fatal(err)
	}
}

func TestPartUpdateAbortsOnMigrationDuringFreshCapture(t *testing.T) {
	d := partDevice(265, 8, 8)
	mock := newMockClient()
	ctrl := newSnapshotController(mock, d)
	s := ctrl.sessions[d.Serial]
	seedSnapshotObservations(s)
	s.observations.observe(s.device, &packets.TileState64{Rect: packets.TileBufferRect{Width: 8}})
	done := make(chan struct{})
	go func() {
		<-mock.sends
		s.updateEndpoint(&net.UDPAddr{IP: net.ParseIP("192.168.1.99"), Port: 56700})
		s.mu.Lock()
		seedSnapshotObservations(s)
		s.observations.observe(s.device, &packets.TileState64{Rect: packets.TileBufferRect{Width: 8}})
		s.mu.Unlock()
		close(done)
	}()
	brightness := 50.0
	err := ctrl.SetLightPartColor(context.Background(), d.Serial, device.LightPartUplight, LightStateUpdate{Brightness: &brightness}, SnapshotOptions{RequireFresh: true, Timeout: time.Second, PollInterval: time.Millisecond})
	<-done
	if !errors.Is(err, ErrEndpointChanged) {
		t.Fatal(err)
	}
	for len(mock.sends) > 0 {
		if msg := <-mock.sends; msg.Type() == 715 || msg.Type() == 716 {
			t.Fatal("migration sent a stale color plan")
		}
	}
}
