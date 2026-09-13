package controller

import (
	"context"
	"math"
	"net"
	"runtime"
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
	complete := receiveDeviceEvent(t, events)
	if complete.Type != DeviceEventSnapshotComplete || complete.Initial || complete.Revision != 0 {
		t.Fatalf("snapshot-complete event = %#v", complete)
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
	if updated.Revision <= complete.Revision {
		t.Fatalf("updated revision = %d, baseline = %d", updated.Revision, complete.Revision)
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
	if event := receiveDeviceEvent(t, first); event.Type != DeviceEventSnapshotComplete {
		t.Fatalf("first initial event = %#v", event)
	}
	if event := receiveDeviceEvent(t, second); event.Type != DeviceEventSnapshotComplete {
		t.Fatalf("second initial event = %#v", event)
	}

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
		if event.Type != DeviceEventAdded || !event.Initial || event.Revision != 0 {
			t.Fatalf("initial event = %#v", event)
		}
	}
	complete := receiveDeviceEvent(t, events)
	if complete.Type != DeviceEventSnapshotComplete || complete.Initial {
		t.Fatalf("snapshot-complete event = %#v", complete)
	}
}

func TestEmptySubscriptionEmitsSnapshotComplete(t *testing.T) {
	mockClient := newMockClient()
	ctrl, err := New(WithClient(mockClient))
	if err != nil {
		t.Fatal(err)
	}
	defer ctrl.Close()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	event := receiveDeviceEvent(t, ctrl.SubscribeDevices(ctx))
	if event.Type != DeviceEventSnapshotComplete || event.Initial || event.Revision != 0 {
		t.Fatalf("initial event = %#v", event)
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
		Revision: 8,
	}, false)
	subscription.initialize([]device.Device{{Serial: device.Serial{1}, Label: "Initial"}}, 7)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go subscription.run(ctx, func() {})

	initial := receiveDeviceEvent(t, subscription.events)
	if initial.Type != DeviceEventAdded || !initial.Initial || initial.Device.Label != "Initial" {
		t.Fatalf("initial event = %#v", initial)
	}
	complete := receiveDeviceEvent(t, subscription.events)
	if complete.Type != DeviceEventSnapshotComplete || complete.Revision != 7 || complete.Initial {
		t.Fatalf("snapshot-complete event = %#v", complete)
	}
	live := receiveDeviceEvent(t, subscription.events)
	if live.Type != DeviceEventUpdated || live.Device.Label != "Updated" || live.Revision != 8 {
		t.Fatalf("live event = %#v", live)
	}
}

func TestSnapshotCompleteCarriesCurrentRevision(t *testing.T) {
	mockClient := newMockClient()
	ctrl, err := New(WithClient(mockClient))
	if err != nil {
		t.Fatal(err)
	}
	defer ctrl.Close()

	ctrl.subscriptionsMu.Lock()
	ctrl.eventRevision = 41
	ctrl.subscriptionsMu.Unlock()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	events := ctrl.SubscribeDevices(ctx)
	complete := receiveDeviceEvent(t, events)
	if complete.Type != DeviceEventSnapshotComplete || complete.Revision != 41 {
		t.Fatalf("snapshot-complete event = %#v", complete)
	}

	ctrl.addSession(&net.UDPAddr{IP: net.IPv4(192, 168, 1, 14)}, device.Serial{3})
	added := receiveDeviceEvent(t, events)
	if added.Type != DeviceEventAdded || added.Revision != 42 || added.Initial {
		t.Fatalf("live added event = %#v", added)
	}
}

func TestSnapshotCompletePrecedesConcurrentUpdate(t *testing.T) {
	mockClient := newMockClient()
	ctrl, err := New(WithClient(mockClient))
	if err != nil {
		t.Fatal(err)
	}
	defer ctrl.Close()

	serial := device.Serial{4}
	ctrl.addSession(&net.UDPAddr{IP: net.IPv4(192, 168, 1, 15)}, serial)
	session := ctrl.sessions[serial]
	session.mu.Lock()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	subscribed := make(chan (<-chan DeviceEvent), 1)
	go func() {
		subscribed <- ctrl.SubscribeDevices(ctx)
	}()

	deadline := time.Now().Add(time.Second)
	for ctrl.mu.TryRLock() {
		ctrl.mu.RUnlock()
		if time.Now().After(deadline) {
			session.mu.Unlock()
			t.Fatal("subscription did not acquire the snapshot boundary")
		}
		runtime.Gosched()
	}

	updated := make(chan struct{})
	go func() {
		ctrl.publishDeviceUpdate(session, DeviceChangeLight)
		close(updated)
	}()
	session.mu.Unlock()

	events := <-subscribed
	if event := receiveDeviceEvent(t, events); event.Type != DeviceEventAdded || !event.Initial {
		t.Fatalf("initial event = %#v", event)
	}
	complete := receiveDeviceEvent(t, events)
	if complete.Type != DeviceEventSnapshotComplete || complete.Revision != 0 {
		t.Fatalf("snapshot-complete event = %#v", complete)
	}
	live := receiveDeviceEvent(t, events)
	if live.Type != DeviceEventUpdated || live.Revision != 1 || live.Revision <= complete.Revision {
		t.Fatalf("live event = %#v", live)
	}
	<-updated
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
	timer := time.NewTimer(time.Second)
	defer timer.Stop()
	for {
		select {
		case _, ok := <-events:
			if !ok {
				return
			}
		case <-timer.C:
			t.Fatal("timed out waiting for device event channel to close")
		}
	}
}
