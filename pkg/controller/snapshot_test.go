package controller

import (
	"context"
	"errors"
	"net"
	"testing"
	"time"

	"github.com/alessio-palumbo/lifxlan-go/pkg/device"
	"github.com/alessio-palumbo/lifxlan-go/pkg/protocol"
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

func TestRestoreStateSnapshotRestoresSingleZoneColorThenPower(t *testing.T) {
	mockClient := newMockClient()
	serial := snapshotSerial(1)
	ctrl := newSnapshotController(mockClient, device.Device{Serial: serial, Address: snapshotAddr(1)})
	snapshot := device.StateSnapshot{Devices: []device.DeviceStateSnapshot{{
		Serial:    serial,
		PoweredOn: false,
		Color:     device.Color{Hue: 120, Saturation: 80, Brightness: 60, Kelvin: 3500},
		LightType: device.LightTypeSingleZone,
	}}}

	if err := ctrl.RestoreStateSnapshot(context.Background(), snapshot, RestoreOptions{Duration: 500 * time.Millisecond}); err != nil {
		t.Fatal(err)
	}

	colorMsg := nextSent(t, mockClient)
	colorPayload, ok := colorMsg.Payload.(*packets.LightSetWaveformOptional)
	if !ok {
		t.Fatalf("first payload = %T, want LightSetWaveformOptional", colorMsg.Payload)
	}
	if !colorPayload.SetHue || !colorPayload.SetSaturation || !colorPayload.SetBrightness || !colorPayload.SetKelvin {
		t.Fatalf("color restore did not set all HSBK fields: %#v", colorPayload)
	}
	if colorPayload.Period != 500 {
		t.Fatalf("color period = %d, want 500", colorPayload.Period)
	}

	powerMsg := nextSent(t, mockClient)
	powerPayload, ok := powerMsg.Payload.(*packets.LightSetPower)
	if !ok {
		t.Fatalf("second payload = %T, want LightSetPower", powerMsg.Payload)
	}
	if powerPayload.Level != 0 || powerPayload.Duration != 500 {
		t.Fatalf("power payload = %#v, want off duration 500", powerPayload)
	}
}

func TestRestoreStateSnapshotRestoresMultiZoneColors(t *testing.T) {
	mockClient := newMockClient()
	serial := snapshotSerial(1)
	ctrl := newSnapshotController(mockClient, device.Device{Serial: serial, Address: snapshotAddr(1)})
	zoneColor := packets.LightHsbk{Hue: 1, Saturation: 2, Brightness: 3, Kelvin: 3500}
	snapshot := device.StateSnapshot{Devices: []device.DeviceStateSnapshot{{
		Serial:    serial,
		PoweredOn: true,
		LightType: device.LightTypeMultiZone,
		Zones:     []packets.LightHsbk{zoneColor},
	}}}

	if err := ctrl.RestoreStateSnapshot(context.Background(), snapshot, RestoreOptions{}); err != nil {
		t.Fatal(err)
	}

	zoneMsg := nextSent(t, mockClient)
	zonePayload, ok := zoneMsg.Payload.(*packets.MultiZoneExtendedSetColorZones)
	if !ok {
		t.Fatalf("first payload = %T, want MultiZoneExtendedSetColorZones", zoneMsg.Payload)
	}
	if zonePayload.Index != 0 || zonePayload.ColorsCount != 1 || zonePayload.Colors[0] != zoneColor {
		t.Fatalf("zone payload = %#v", zonePayload)
	}

	powerMsg := nextSent(t, mockClient)
	powerPayload, ok := powerMsg.Payload.(*packets.DeviceSetPower)
	if !ok {
		t.Fatalf("second payload = %T, want DeviceSetPower", powerMsg.Payload)
	}
	if powerPayload.Level != 65535 {
		t.Fatalf("power level = %d, want 65535", powerPayload.Level)
	}
}

func TestRestoreStateSnapshotRestoresMatrixChains(t *testing.T) {
	mockClient := newMockClient()
	serial := snapshotSerial(1)
	ctrl := newSnapshotController(mockClient, device.Device{Serial: serial, Address: snapshotAddr(1)})
	colors := make([]packets.LightHsbk, 64)
	colors[0] = packets.LightHsbk{Hue: 1, Saturation: 2, Brightness: 3, Kelvin: 3500}
	snapshot := device.StateSnapshot{Devices: []device.DeviceStateSnapshot{{
		Serial:       serial,
		PoweredOn:    true,
		LightType:    device.LightTypeMatrix,
		MatrixChains: [][]packets.LightHsbk{colors},
	}}}

	if err := ctrl.RestoreStateSnapshot(context.Background(), snapshot, RestoreOptions{}); err != nil {
		t.Fatal(err)
	}

	matrixMsg := nextSent(t, mockClient)
	matrixPayload, ok := matrixMsg.Payload.(*packets.TileSet64)
	if !ok {
		t.Fatalf("first payload = %T, want TileSet64", matrixMsg.Payload)
	}
	if matrixPayload.TileIndex != 0 || matrixPayload.Length != 1 || matrixPayload.Rect.Width != 8 || matrixPayload.Colors[0] != colors[0] {
		t.Fatalf("matrix payload = %#v", matrixPayload)
	}
	assertSentPayload(t, mockClient, uint16(packets.PayloadTypeDeviceSetPower))
}

func TestRestoreStateSnapshotRepeatsAttempts(t *testing.T) {
	mockClient := newMockClient()
	serial := snapshotSerial(1)
	ctrl := newSnapshotController(mockClient, device.Device{Serial: serial, Address: snapshotAddr(1)})
	snapshot := device.StateSnapshot{Devices: []device.DeviceStateSnapshot{{
		Serial:    serial,
		PoweredOn: true,
		Color:     device.Color{Kelvin: 3500},
		LightType: device.LightTypeSingleZone,
	}}}

	if err := ctrl.RestoreStateSnapshot(context.Background(), snapshot, RestoreOptions{Attempts: 2, RetryDelay: time.Millisecond}); err != nil {
		t.Fatal(err)
	}

	if got := sentCount(mockClient); got != 4 {
		t.Fatalf("sent messages = %d, want 4", got)
	}
}

func TestRestoreStateSnapshotHonorsContextCancellation(t *testing.T) {
	mockClient := newMockClient()
	serial := snapshotSerial(1)
	ctrl := newSnapshotController(mockClient, device.Device{Serial: serial, Address: snapshotAddr(1)})
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	err := ctrl.RestoreStateSnapshot(ctx, device.StateSnapshot{Devices: []device.DeviceStateSnapshot{{Serial: serial}}}, RestoreOptions{})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("error = %v, want context.Canceled", err)
	}
	if got := sentCount(mockClient); got != 0 {
		t.Fatalf("sent messages = %d, want 0", got)
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

func nextSent(t *testing.T, mockClient *mockClient) *protocol.Message {
	t.Helper()
	select {
	case msg := <-mockClient.sends:
		return msg
	case <-time.After(20 * time.Millisecond):
		t.Fatal("timed out waiting for sent message")
	}
	return nil
}

func sentCount(mockClient *mockClient) int {
	count := 0
	for {
		select {
		case <-mockClient.sends:
			count++
		default:
			return count
		}
	}
}
