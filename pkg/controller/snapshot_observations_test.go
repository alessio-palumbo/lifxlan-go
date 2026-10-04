package controller

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/alessio-palumbo/lifxlan-go/pkg/device"
	"github.com/alessio-palumbo/lifxlan-go/pkg/protocol"
	"github.com/alessio-palumbo/lifxprotocol-go/gen/protocol/packets"
)

func snapshotReceiver(t *testing.T) (*Controller, *deviceSession, *mockClient) {
	t.Helper()
	sender := newMockClient()
	serial := snapshotSerial(1)
	ctrl := newSnapshotController(sender, device.Device{Serial: serial, Address: snapshotAddr(1)})
	session := ctrl.sessions[serial]
	session.inbound = make(chan *protocol.Message)
	go session.recvloop()
	t.Cleanup(session.close)
	return ctrl, session, sender
}

func receiveSnapshotState(t *testing.T, session *deviceSession, payload packets.Payload) {
	t.Helper()
	before := session.deviceSnapshot().LastSeenAt
	session.inbound <- protocol.NewMessage(payload)
	deadline := time.Now().Add(time.Second)
	for !session.deviceSnapshot().LastSeenAt.After(before) {
		if time.Now().After(deadline) {
			t.Fatal("response was not processed")
		}
		time.Sleep(time.Millisecond)
	}
}

func assertSnapshotMissing(t *testing.T, ctrl *Controller, fresh bool, reason string) {
	t.Helper()
	_, err := ctrl.CaptureStateSnapshot(context.Background(), []device.Serial{snapshotSerial(1)}, SnapshotOptions{
		Timeout: 5 * time.Millisecond, RequireFresh: fresh,
	})
	if err == nil || !strings.Contains(err.Error(), reason) {
		t.Fatalf("capture error = %v, want missing %s", err, reason)
	}
}

func TestCaptureRequiresObservedProductAndLight(t *testing.T) {
	ctrl, session, _ := snapshotReceiver(t)
	assertSnapshotMissing(t, ctrl, false, "product information")
	receiveSnapshotState(t, session, &packets.DeviceStateVersion{Product: 55})
	assertSnapshotMissing(t, ctrl, false, "light state")
}

func TestCaptureMatrixRequiresCompleteObservedPixelsRegardlessOfPower(t *testing.T) {
	for _, poweredOn := range []bool{false, true} {
		name := "off"
		if poweredOn {
			name = "on"
		}
		t.Run(name, func(t *testing.T) {
			ctrl, session, _ := snapshotReceiver(t)
			receiveSnapshotState(t, session, &packets.DeviceStateVersion{Product: 55})
			var power uint16
			if poweredOn {
				power = 65535
			}
			receiveSnapshotState(t, session, &packets.LightState{Power: power})
			receiveSnapshotState(t, session, &packets.TileStateDeviceChain{
				TileDevicesCount: 2,
				TileDevices:      [16]packets.TileStateDevice{{Width: 16, Height: 20}, {Width: 16, Height: 20}},
			})
			// Metadata has allocated ten black pixel blocks, none observed yet.
			assertSnapshotMissing(t, ctrl, false, "matrix coverage")
			// Out-of-order and duplicate blocks must not substitute for missing ones.
			for _, chain := range []uint8{1, 0} {
				for _, y := range []uint8{16, 8, 0, 4, 4} {
					receiveSnapshotState(t, session, &packets.TileState64{
						TileIndex: chain, Rect: packets.TileBufferRect{Width: 16, Y: y},
					})
				}
			}
			assertSnapshotMissing(t, ctrl, false, "matrix coverage")
			for chain := range 2 {
				receiveSnapshotState(t, session, &packets.TileState64{
					TileIndex: uint8(chain), Rect: packets.TileBufferRect{Width: 16, Y: 12},
				})
			}
			snapshot, err := ctrl.CaptureStateSnapshot(context.Background(), []device.Serial{snapshotSerial(1)}, SnapshotOptions{})
			if err != nil {
				t.Fatal(err)
			}
			if snapshot.Devices[0].PoweredOn != poweredOn || len(snapshot.Devices[0].MatrixChains[0]) != 320 {
				t.Fatalf("unexpected snapshot: %+v", snapshot.Devices[0])
			}
			assertSnapshotMissing(t, ctrl, true, "light state")
			baselines := make(map[device.Serial]snapshotBaseline)
			selected := []device.Serial{snapshotSerial(1)}
			ctrl.snapshotProgress(selected, baselines, true, false)
			receiveSnapshotState(t, session, &packets.LightState{Power: power})
			// New light state and one pixel packet cannot refresh the whole matrix.
			receiveSnapshotState(t, session, &packets.TileState64{Rect: packets.TileBufferRect{Width: 16}})
			if _, missing, _ := ctrl.snapshotProgress(selected, baselines, true, false); len(missing) == 0 {
				t.Fatal("fresh capture accepted old pixel coverage")
			}
			for chain := range 2 {
				for block := range 5 {
					receiveSnapshotState(t, session, &packets.TileState64{
						TileIndex: uint8(chain), Rect: packets.TileBufferRect{Width: 16, Y: uint8(block * 4)},
					})
				}
			}
			if _, missing, _ := ctrl.snapshotProgress(selected, baselines, true, false); len(missing) != 0 {
				t.Fatalf("fresh black matrix remained incomplete: %v", missing)
			}
			// A changed shape invalidates previously received coverage.
			receiveSnapshotState(t, session, &packets.TileStateDeviceChain{
				TileDevicesCount: 1, TileDevices: [16]packets.TileStateDevice{{Width: 8, Height: 8}},
			})
			assertSnapshotMissing(t, ctrl, false, "matrix coverage")
		})
	}
}

func TestCaptureFreshAcceptsUnchangedLightResponseWithoutEvent(t *testing.T) {
	ctrl, session, sender := snapshotReceiver(t)
	changes := make(chan DeviceChange, 10)
	session.onUpdate = func(_ *deviceSession, change DeviceChange) { changes <- change }
	receiveSnapshotState(t, session, &packets.DeviceStateVersion{Product: 1})
	receiveSnapshotState(t, session, &packets.LightState{})
	// Flush initial product changes; the repeated light state is identical.
	for len(changes) > 0 {
		<-changes
	}
	assertSnapshotMissing(t, ctrl, true, "light state")
	for len(sender.sends) > 0 {
		<-sender.sends
	}
	done := make(chan error, 1)
	go func() {
		_, err := ctrl.CaptureStateSnapshot(context.Background(), []device.Serial{snapshotSerial(1)}, SnapshotOptions{
			Timeout: time.Second, RequireFresh: true,
		})
		done <- err
	}()
	assertSentPayload(t, sender, uint16(packets.PayloadTypeLightGet))
	receiveSnapshotState(t, session, &packets.LightState{})
	if err := <-done; err != nil {
		t.Fatal(err)
	}
	if len(changes) != 0 {
		t.Fatal("unchanged observation emitted a value-change event")
	}
}

func TestCaptureMultizoneRequiresEveryReceivedRange(t *testing.T) {
	ctrl, session, _ := snapshotReceiver(t)
	receiveSnapshotState(t, session, &packets.DeviceStateVersion{Product: 214})
	receiveSnapshotState(t, session, &packets.LightState{})
	receiveSnapshotState(t, session, &packets.MultiZoneExtendedStateMultiZone{Count: 120, Index: 82, ColorsCount: 38})
	assertSnapshotMissing(t, ctrl, false, "multizone coverage")
	receiveSnapshotState(t, session, &packets.MultiZoneExtendedStateMultiZone{Count: 120, ColorsCount: 82})
	if _, err := ctrl.CaptureStateSnapshot(context.Background(), []device.Serial{snapshotSerial(1)}, SnapshotOptions{}); err != nil {
		t.Fatal(err)
	}
	// Expanding a zone count discards coverage for the previous geometry.
	receiveSnapshotState(t, session, &packets.MultiZoneExtendedStateMultiZone{Count: 121, ColorsCount: 82})
	assertSnapshotMissing(t, ctrl, false, "multizone coverage")
}

func TestFreshCaptureBaselinesAreIndependentAndResetOnReplacement(t *testing.T) {
	ctrl, session, sender := snapshotReceiver(t)
	receiveSnapshotState(t, session, &packets.DeviceStateVersion{Product: 1})
	receiveSnapshotState(t, session, &packets.LightState{})
	selected := []device.Serial{snapshotSerial(1)}
	first := make(map[device.Serial]snapshotBaseline)
	second := make(map[device.Serial]snapshotBaseline)
	ctrl.snapshotProgress(selected, first, true, false)
	receiveSnapshotState(t, session, &packets.LightState{})
	ctrl.snapshotProgress(selected, second, true, false)
	if _, missing, _ := ctrl.snapshotProgress(selected, first, true, false); len(missing) != 0 {
		t.Fatalf("later capture invalidated earlier baseline: %v", missing)
	}
	if _, missing, _ := ctrl.snapshotProgress(selected, second, true, false); len(missing) == 0 {
		t.Fatal("later capture accepted earlier observation")
	}
	// A new session starts its own generation sequence. Never reuse the old
	// session's baseline or completeness when a serial reconnects.
	replacement := newSnapshotController(sender, device.Device{Serial: selected[0], Address: snapshotAddr(1)}).sessions[selected[0]]
	seedSnapshotObservations(replacement)
	ctrl.mu.Lock()
	ctrl.sessions[selected[0]] = replacement
	ctrl.mu.Unlock()
	if _, missing, _ := ctrl.snapshotProgress(selected, first, true, false); len(missing) == 0 {
		t.Fatal("replacement session incorrectly inherited fresh readiness")
	}
	if first[selected[0]].session != replacement {
		t.Fatal("baseline still refers to previous session")
	}
}
