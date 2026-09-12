package controller

import (
	"context"
	"math"
	"net"
	"testing"
	"time"

	"github.com/alessio-palumbo/lifxlan-go/pkg/device"
	"github.com/alessio-palumbo/lifxlan-go/pkg/protocol"
	"github.com/alessio-palumbo/lifxprotocol-go/gen/protocol/packets"
)

func TestSubscribeDevicesStreamsInitialAndObservedState(t *testing.T) {
	mockClient := newMockClient()
	ctrl, err := New(WithClient(mockClient))
	if err != nil {
		t.Fatal(err)
	}
	defer ctrl.Close()

	addr := &net.UDPAddr{IP: net.IPv4(192, 168, 1, 10), Port: 56700}
	serial := device.Serial{1}
	ctrl.addSession(addr, serial)
	session := ctrl.sessions[serial]
	session.mu.Lock()
	session.device.Label = "Desk"
	session.mu.Unlock()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	events := ctrl.SubscribeDevices(ctx)

	initial := receiveDeviceEvent(t, events)
	if initial.Type != DeviceEventAdded || !initial.Initial {
		t.Fatalf("initial event = %#v", initial)
	}
	if initial.Device.Serial != serial || initial.Device.Label != "Desk" {
		t.Fatalf("initial device = %#v", initial.Device)
	}

	session.inbound <- protocol.NewMessage(&packets.LightState{
		Color: packets.LightHsbk{Brightness: math.MaxUint16, Kelvin: 3500},
		Power: math.MaxUint16,
	})
	updated := receiveDeviceEvent(t, events)
	if updated.Type != DeviceEventUpdated || updated.Changes != DeviceChangeLight {
		t.Fatalf("updated event = %#v", updated)
	}
	if !updated.Device.PoweredOn || updated.Device.Color.Brightness != 100 {
		t.Fatalf("updated light = %#v", updated.Device)
	}
	if updated.Revision <= initial.Revision {
		t.Fatalf("updated revision = %d, initial = %d", updated.Revision, initial.Revision)
	}

	// An unchanged response must not emit an event. The following group update
	// will therefore be the next event in the stream.
	session.inbound <- protocol.NewMessage(&packets.LightState{
		Color: packets.LightHsbk{Brightness: math.MaxUint16, Kelvin: 3500},
		Power: math.MaxUint16,
	})
	session.inbound <- protocol.NewMessage(&packets.DeviceStateGroup{
		Group: [16]byte{2},
		Label: [32]byte{'O', 'f', 'f', 'i', 'c', 'e'},
	})
	group := receiveDeviceEvent(t, events)
	if group.Changes != DeviceChangeGroup || group.Device.Group != "Office" {
		t.Fatalf("event after unchanged light state = %#v", group)
	}

	ctrl.terminateSession(serial)
	removed := receiveDeviceEvent(t, events)
	if removed.Type != DeviceEventRemoved || removed.Device.Serial != serial {
		t.Fatalf("removed event = %#v", removed)
	}
	if removed.Revision <= group.Revision {
		t.Fatalf("removed revision = %d, group = %d", removed.Revision, group.Revision)
	}
}

func TestSubscribeDevicesIsolatesSubscribers(t *testing.T) {
	mockClient := newMockClient()
	ctrl, err := New(WithClient(mockClient))
	if err != nil {
		t.Fatal(err)
	}
	defer ctrl.Close()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	first := ctrl.SubscribeDevices(ctx)
	second := ctrl.SubscribeDevices(ctx)

	addr := &net.UDPAddr{IP: net.IPv4(192, 168, 1, 11), Port: 56700}
	serial := device.Serial{2}
	ctrl.addSession(addr, serial)

	firstEvent := receiveDeviceEvent(t, first)
	secondEvent := receiveDeviceEvent(t, second)
	firstEvent.Device.Address.IP[0] = 10
	if secondEvent.Device.Address.IP[0] == 10 {
		t.Fatal("subscribers share event device storage")
	}
	if ctrl.GetDevices()[0].Address.IP[0] == 10 {
		t.Fatal("event shares device storage with the controller cache")
	}
}

func TestInitialDevicesDoNotOverflowLiveEventBuffer(t *testing.T) {
	mockClient := newMockClient()
	ctrl, err := New(WithClient(mockClient))
	if err != nil {
		t.Fatal(err)
	}
	defer ctrl.Close()

	ctrl.addSession(&net.UDPAddr{IP: net.IPv4(192, 168, 1, 12)}, device.Serial{1})
	ctrl.addSession(&net.UDPAddr{IP: net.IPv4(192, 168, 1, 13)}, device.Serial{2})

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	events := ctrl.SubscribeDevices(ctx, WithSubscriptionBufferSize(1))

	for range 2 {
		event := receiveDeviceEvent(t, events)
		if event.Type != DeviceEventAdded || !event.Initial {
			t.Fatalf("initial event = %#v", event)
		}
	}
}

func TestSubscriptionOverflowRequiresResync(t *testing.T) {
	subscription := newDeviceSubscription(1)
	subscription.enqueue(DeviceEvent{Type: DeviceEventAdded, Revision: 1}, false)
	subscription.enqueue(DeviceEvent{Type: DeviceEventUpdated, Revision: 2}, false)
	subscription.enqueue(DeviceEvent{Type: DeviceEventRemoved, Revision: 3}, false)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go subscription.run(ctx, func() {})

	event := receiveDeviceEvent(t, subscription.events)
	if event.Type != DeviceEventResyncRequired || event.Revision != 3 {
		t.Fatalf("overflow event = %#v", event)
	}
}

func TestSubscriptionOrdersInitialDevicesBeforeQueuedLiveEvents(t *testing.T) {
	subscription := newDeviceSubscription(1)
	subscription.enqueue(DeviceEvent{
		Type:     DeviceEventUpdated,
		Device:   device.Device{Serial: device.Serial{1}, Label: "Updated"},
		Changes:  DeviceChangeLabel,
		Revision: 1,
	}, false)
	subscription.initialize([]device.Device{{Serial: device.Serial{1}, Label: "Initial"}})

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go subscription.run(ctx, func() {})

	initial := receiveDeviceEvent(t, subscription.events)
	if initial.Type != DeviceEventAdded || !initial.Initial || initial.Device.Label != "Initial" {
		t.Fatalf("initial event = %#v", initial)
	}
	live := receiveDeviceEvent(t, subscription.events)
	if live.Type != DeviceEventUpdated || live.Device.Label != "Updated" {
		t.Fatalf("live event = %#v", live)
	}
}

func TestSubscriptionClosesWithContextAndController(t *testing.T) {
	t.Run("context", func(t *testing.T) {
		mockClient := newMockClient()
		ctrl, err := New(WithClient(mockClient))
		if err != nil {
			t.Fatal(err)
		}
		defer ctrl.Close()

		ctx, cancel := context.WithCancel(context.Background())
		events := ctrl.SubscribeDevices(ctx)
		cancel()
		requireDeviceEventsClosed(t, events)
	})

	t.Run("controller", func(t *testing.T) {
		mockClient := newMockClient()
		ctrl, err := New(WithClient(mockClient))
		if err != nil {
			t.Fatal(err)
		}

		events := ctrl.SubscribeDevices(context.Background())
		if err := ctrl.Close(); err != nil {
			t.Fatal(err)
		}
		requireDeviceEventsClosed(t, events)
	})
}

func receiveDeviceEvent(t *testing.T, events <-chan DeviceEvent) DeviceEvent {
	t.Helper()
	select {
	case event, ok := <-events:
		if !ok {
			t.Fatal("device event channel closed")
		}
		return event
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for device event")
		return DeviceEvent{}
	}
}

func requireDeviceEventsClosed(t *testing.T, events <-chan DeviceEvent) {
	t.Helper()
	select {
	case _, ok := <-events:
		if ok {
			t.Fatal("device event channel remains open")
		}
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for device event channel to close")
	}
}
