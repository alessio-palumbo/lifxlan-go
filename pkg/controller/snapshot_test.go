package controller

import (
	"context"
	"errors"
	"net"
	"testing"
	"time"

	"github.com/alessio-palumbo/lifxlan-go/pkg/device"
	"github.com/alessio-palumbo/lifxprotocol-go/gen/protocol/packets"
)

func TestCaptureStateSnapshotCapturesReadyDevices(t *testing.T) {
	mockClient := newMockClient()
	serial := snapshotSerial(1)
	zoneColor := packets.LightHsbk{Hue: 1, Saturation: 2, Brightness: 3, Kelvin: 3500}
	ctrl := newSnapshotController(mockClient, device.Device{
		Serial:    serial,
		Address:   snapshotAddr(1),
		PoweredOn: true,
		Color:     device.Color{Hue: 120, Saturation: 80, Brightness: 60, Kelvin: 3500},
		LightType: device.LightTypeMultiZone,
		MultizoneProperties: device.MultizoneProperties{
			Zones: []packets.LightHsbk{zoneColor},
		},
	})

	snapshot, err := ctrl.CaptureStateSnapshot(context.Background(), []device.Serial{serial}, SnapshotOptions{
		Timeout:      50 * time.Millisecond,
		PollInterval: time.Millisecond,
	})
	if err != nil {
		t.Fatal(err)
	}

	if len(snapshot.Devices) != 1 {
		t.Fatalf("snapshot devices = %d, want 1", len(snapshot.Devices))
	}
	if snapshot.Devices[0].Serial != serial || snapshot.Devices[0].Color.Hue != 120 {
		t.Fatalf("snapshot = %#v", snapshot.Devices[0])
	}
	if len(snapshot.Devices[0].Zones) != 1 || snapshot.Devices[0].Zones[0] != zoneColor {
		t.Fatalf("zones = %#v, want %#v", snapshot.Devices[0].Zones, []packets.LightHsbk{zoneColor})
	}
	assertSentPayload(t, mockClient, uint16(packets.PayloadTypeLightGet))
	assertSentPayload(t, mockClient, uint16(packets.PayloadTypeDeviceGetPower))
	assertSentPayload(t, mockClient, uint16(packets.PayloadTypeMultiZoneExtendedGetColorZones))
}

func TestCaptureStateSnapshotPreservesSerialOrderAndDeDuplicates(t *testing.T) {
	mockClient := newMockClient()
	serial0 := snapshotSerial(1)
	serial1 := snapshotSerial(2)
	ctrl := newSnapshotController(mockClient,
		device.Device{Serial: serial0, Address: snapshotAddr(1), Label: "A", Color: device.Color{Hue: 10}},
		device.Device{Serial: serial1, Address: snapshotAddr(2), Label: "B", Color: device.Color{Hue: 20}},
	)

	snapshot, err := ctrl.CaptureStateSnapshot(context.Background(), []device.Serial{serial1, serial0, serial1}, SnapshotOptions{
		Timeout:      50 * time.Millisecond,
		PollInterval: time.Millisecond,
	})
	if err != nil {
		t.Fatal(err)
	}

	if len(snapshot.Devices) != 2 {
		t.Fatalf("snapshot devices = %d, want 2", len(snapshot.Devices))
	}
	if snapshot.Devices[0].Serial != serial1 || snapshot.Devices[1].Serial != serial0 {
		t.Fatalf("snapshot serial order = %#v, want [%s %s]", snapshot.Devices, serial1, serial0)
	}
}

func TestCaptureStateSnapshotWaitsForPoweredOnMatrixState(t *testing.T) {
	mockClient := newMockClient()
	serial := snapshotSerial(1)
	ctrl := newSnapshotController(mockClient, device.Device{
		Serial:    serial,
		Address:   snapshotAddr(1),
		PoweredOn: true,
		LightType: device.LightTypeMatrix,
		MatrixProperties: device.MatrixProperties{
			Width:        8,
			Height:       8,
			ChainLength:  1,
			StatePackets: 1,
		},
	})

	done := make(chan error, 1)
	go func() {
		snapshot, err := ctrl.CaptureStateSnapshot(context.Background(), []device.Serial{serial}, SnapshotOptions{
			Timeout:      100 * time.Millisecond,
			PollInterval: 5 * time.Millisecond,
		})
		if err == nil && len(snapshot.Devices[0].MatrixChains) != 1 {
			err = errors.New("snapshot did not include matrix chain colors")
		}
		done <- err
	}()

	assertSentPayload(t, mockClient, uint16(packets.PayloadTypeTileGet64))
	session := ctrl.sessions[serial]
	session.mu.Lock()
	session.device.MatrixProperties.ChainZones = [][]packets.LightHsbk{{{Hue: 1, Saturation: 2, Brightness: 3, Kelvin: 3500}}}
	session.mu.Unlock()

	select {
	case err := <-done:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(150 * time.Millisecond):
		t.Fatal("CaptureStateSnapshot did not return after matrix state became ready")
	}
}

func TestCaptureStateSnapshotRequestsMatrixChainBeforePixels(t *testing.T) {
	mockClient := newMockClient()
	serial := snapshotSerial(1)
	ctrl := newSnapshotController(mockClient, device.Device{
		Serial:    serial,
		Address:   snapshotAddr(1),
		PoweredOn: true,
		LightType: device.LightTypeMatrix,
	})

	_, err := ctrl.CaptureStateSnapshot(context.Background(), []device.Serial{serial}, SnapshotOptions{
		Timeout:      2 * time.Millisecond,
		PollInterval: time.Millisecond,
	})
	if err == nil {
		t.Fatal("CaptureStateSnapshot should time out without matrix chain state")
	}

	assertSentPayload(t, mockClient, uint16(packets.PayloadTypeTileGetDeviceChain))
	assertNotSentPayload(t, mockClient, uint16(packets.PayloadTypeTileGet64))
}

func TestCaptureStateSnapshotTimesOutWhenDeviceIsMissing(t *testing.T) {
	ctrl := newSnapshotController(newMockClient())

	_, err := ctrl.CaptureStateSnapshot(context.Background(), []device.Serial{snapshotSerial(1)}, SnapshotOptions{
		Timeout:      time.Millisecond,
		PollInterval: time.Millisecond,
	})
	if err == nil {
		t.Fatal("CaptureStateSnapshot should time out for a missing device")
	}
}

func TestCaptureStateSnapshotHonorsContextCancellation(t *testing.T) {
	ctrl := newSnapshotController(newMockClient())
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := ctrl.CaptureStateSnapshot(ctx, []device.Serial{snapshotSerial(1)}, SnapshotOptions{
		Timeout:      time.Second,
		PollInterval: time.Millisecond,
	})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("error = %v, want context.Canceled", err)
	}
}

func TestCaptureStateSnapshotRejectsEmptySerials(t *testing.T) {
	ctrl := newSnapshotController(newMockClient())

	_, err := ctrl.CaptureStateSnapshot(context.Background(), nil, SnapshotOptions{})
	if err == nil {
		t.Fatal("CaptureStateSnapshot should reject empty serials")
	}
}

func newSnapshotController(mockClient *mockClient, devices ...device.Device) *Controller {
	ctrl := &Controller{
		client:   mockClient,
		logger:   discardLogger(),
		sessions: make(map[device.Serial]*deviceSession),
	}
	for i := range devices {
		d := devices[i]
		session := &deviceSession{
			sender: mockClient,
			logger: discardLogger(),
			device: &d,
			done:   make(chan struct{}),
		}
		ctrl.sessions[d.Serial] = session
	}
	return ctrl
}

func snapshotSerial(value byte) device.Serial {
	return device.Serial([8]byte{value, 0, 0, 0, 0, 0, 0, 0})
}

func snapshotAddr(value byte) *net.UDPAddr {
	return &net.UDPAddr{IP: net.IPv4(192, 168, 0, value)}
}

func assertSentPayload(t *testing.T, mockClient *mockClient, payloadType uint16) {
	t.Helper()
	if !waitSentPayload(mockClient, payloadType, 20*time.Millisecond) {
		t.Fatalf("payload %d was not sent", payloadType)
	}
}

func assertNotSentPayload(t *testing.T, mockClient *mockClient, payloadType uint16) {
	t.Helper()
	if sentPayload(mockClient, payloadType) {
		t.Fatalf("payload %d was sent", payloadType)
	}
}

func sentPayload(mockClient *mockClient, payloadType uint16) bool {
	for {
		select {
		case msg := <-mockClient.sends:
			if msg.Type() == payloadType {
				return true
			}
		default:
			return false
		}
	}
}

func waitSentPayload(mockClient *mockClient, payloadType uint16, timeout time.Duration) bool {
	deadline := time.NewTimer(timeout)
	defer deadline.Stop()
	for {
		select {
		case msg := <-mockClient.sends:
			if msg.Type() == payloadType {
				return true
			}
		case <-deadline.C:
			return false
		}
	}
}
