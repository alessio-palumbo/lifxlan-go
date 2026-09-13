package control

import (
	"context"
	"errors"
	"reflect"
	"testing"

	"github.com/alessio-palumbo/lifxlan-go/pkg/controller"
	"github.com/alessio-palumbo/lifxlan-go/pkg/device"
)

func TestApplyStateResolvesSelectorsAndReportsPerDeviceResults(t *testing.T) {
	devices := controlDevices(t)
	partial := &controller.StateSendError{Sent: 1, Total: 2, Err: errors.New("send failed")}
	backend := &fakeBackend{
		devices: devices,
		errors: map[device.Serial]error{
			devices[1].Serial: partial,
			devices[2].Serial: controller.ErrUnsupportedCapability,
		},
	}
	service := New(backend)

	results, err := service.ApplyState([]string{"group:lounge", "serial:" + devices[1].Serial.String()}, controller.StateUpdate{
		Relays: []controller.RelayStateUpdate{{Index: 0, PoweredOn: true}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if got, want := backend.called, []device.Serial{devices[0].Serial, devices[2].Serial, devices[1].Serial}; !reflect.DeepEqual(got, want) {
		t.Fatalf("called = %v, want %v", got, want)
	}
	if got, want := []ApplyStatus{results[0].Status, results[1].Status, results[2].Status}, []ApplyStatus{ApplyAccepted, ApplyRejected, ApplyPartial}; !reflect.DeepEqual(got, want) {
		t.Fatalf("statuses = %v, want %v", got, want)
	}
	if results[2].Sent != 1 || results[2].Total != 2 {
		t.Fatalf("partial result = %+v", results[2])
	}
}

func TestApplyStateRejectsInvalidAndUnmatchedSelectors(t *testing.T) {
	service := New(&fakeBackend{devices: controlDevices(t)})

	for _, selectors := range [][]string{nil, {}, {"unknown:value"}, {"serial:nope"}} {
		_, err := service.ApplyState(selectors, controller.StateUpdate{})
		if !errors.Is(err, ErrInvalidSelectors) {
			t.Errorf("selectors %v error = %v, want ErrInvalidSelectors", selectors, err)
		}
	}
	if _, err := service.ApplyState([]string{"label:missing"}, controller.StateUpdate{}); !errors.Is(err, ErrNoDevicesMatched) {
		t.Fatalf("unmatched error = %v, want ErrNoDevicesMatched", err)
	}
	if _, err := service.ApplyState([]string{"all"}, controller.StateUpdate{}); !errors.Is(err, ErrInvalidState) {
		t.Fatalf("invalid state error = %v, want ErrInvalidState", err)
	}
}

func TestGetDevice(t *testing.T) {
	devices := controlDevices(t)
	service := New(&fakeBackend{devices: devices})

	got, ok := service.GetDevice(devices[1].Serial)
	if !ok || got.Serial != devices[1].Serial {
		t.Fatalf("device = %+v, ok = %t", got, ok)
	}
	if _, ok := service.GetDevice(device.Serial{0xff}); ok {
		t.Fatal("missing device was found")
	}
}

type fakeBackend struct {
	devices []device.Device
	errors  map[device.Serial]error
	called  []device.Serial
}

func (b *fakeBackend) GetDevice(serial device.Serial) (device.Device, bool) {
	for _, d := range b.devices {
		if d.Serial == serial {
			return d.Clone(), true
		}
	}
	return device.Device{}, false
}

func (b *fakeBackend) GetDevices() []device.Device {
	return append([]device.Device(nil), b.devices...)
}

func (b *fakeBackend) SetState(serial device.Serial, _ controller.StateUpdate) error {
	b.called = append(b.called, serial)
	return b.errors[serial]
}

func (b *fakeBackend) SubscribeDevices(context.Context, ...controller.SubscriptionOption) <-chan controller.DeviceEvent {
	return make(chan controller.DeviceEvent)
}

func controlDevices(t *testing.T) []device.Device {
	t.Helper()
	serials := make([]device.Serial, 3)
	for i, value := range []string{"001122334455", "aabbccddeeff", "112233445566"} {
		serial, err := device.SerialFromHex(value)
		if err != nil {
			t.Fatal(err)
		}
		serials[i] = serial
	}
	return []device.Device{
		{Serial: serials[0], Label: "TV", Group: "Lounge"},
		{Serial: serials[1], Label: "Desk", Group: "Office"},
		{Serial: serials[2], Label: "Neon", Group: "Lounge"},
	}
}
