package controller

import (
	"math"
	"net"
	"slices"
	"testing"
	"time"

	"github.com/alessio-palumbo/lifxlan-go/pkg/device"
	"github.com/alessio-palumbo/lifxlan-go/pkg/protocol"
	"github.com/alessio-palumbo/lifxprotocol-go/gen/protocol/packets"
	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSession(t *testing.T) {
	var (
		addr0   = &net.UDPAddr{IP: net.IPv4(192, 168, 0, 10)}
		serial0 = device.Serial([8]byte{1, 0, 0, 0, 0, 0, 0, 0})

		cfg0 = &Config{
			discoveryPeriod:                 defaultDiscoveryPeriod,
			highFrequencyStateRefreshPeriod: defaulthighFrequencyStateRefreshPeriod,
			lowFrequencyStateRefreshPeriod:  defaultlowFrequencyStateRefreshPeriod,
		}
	)

	t.Run("Sends initial state messages", func(t *testing.T) {
		mockClient := newMockClient()
		session, err := NewDeviceSession(addr0, serial0, mockClient, cfg0)
		require.NoError(t, err)

		var gotMsgs []packets.Payload
	outer:
		for {
			select {
			case msg := <-mockClient.sends:
				gotMsgs = append(gotMsgs, msg.Payload)
			case <-time.After(10 * time.Millisecond):
				break outer
			}
		}

		wantMsgs := []packets.Payload{}
		for _, p := range requiredStateMessages() {
			wantMsgs = append(wantMsgs, p.Payload)
		}
		assert.Equal(t, wantMsgs, gotMsgs)
		session.Close()
	})

	t.Run("It sends high frequency messages", func(t *testing.T) {
		cfg := *cfg0
		cfg.highFrequencyStateRefreshPeriod = time.Millisecond
		mockClient := newMockClient()
		session, err := NewDeviceSession(addr0, serial0, mockClient, &cfg)
		require.NoError(t, err)

		var gotMsgs int
		timeout := time.After(10 * time.Millisecond)
	outer:
		for {
			select {
			case msg := <-mockClient.sends:
				if msg.Payload == (&packets.LightGet{}) {
					gotMsgs++
				}
			case <-timeout:
				break outer
			}
		}

		assert.Greater(t, 5, gotMsgs)
		session.Close()
	})

	t.Run("It sends low frequency messages", func(t *testing.T) {
		cfg := *cfg0
		cfg.lowFrequencyStateRefreshPeriod = time.Millisecond
		mockClient := newMockClient()
		session, err := NewDeviceSession(addr0, serial0, mockClient, &cfg)
		require.NoError(t, err)

		var gotMsgs []packets.Payload

		timeout := time.After(10 * time.Millisecond)
	outer:
		for {
			select {
			case msg := <-mockClient.sends:
				gotMsgs = append(gotMsgs, msg.Payload)
			case <-timeout:
				break outer
			}
		}

		var lowFreqMsgs []packets.Payload
		for msg := range slices.Values(session.device.LowFreqStateMessages()) {
			lowFreqMsgs = append(lowFreqMsgs, msg.Payload)
		}
		assert.Subset(t, gotMsgs, lowFreqMsgs)
		session.Close()
	})

	t.Run("Updates state", func(t *testing.T) {
		mockClient := newMockClient()
		session, err := NewDeviceSession(addr0, serial0, mockClient, cfg0)
		require.NoError(t, err)

		wantDevice := device.Device{
			Serial: device.Serial(serial0), Address: addr0,
		}
		deviceSnapshot := session.DeviceSnapshot()
		assert.Equal(t, wantDevice, deviceSnapshot)
		assert.Equal(t, deviceSnapshot.LastSeenAt, time.Time{})

		// Updates label
		session.inbound <- protocol.NewMessage(&packets.DeviceStateLabel{Label: [32]byte{'L', 'i', 'f', 'y'}})
		time.Sleep(10 * time.Millisecond)
		assert.Equal(t, "Lify", session.DeviceSnapshot().Label)

		// Updates light state
		color := packets.LightHsbk{Hue: 0, Saturation: 0, Kelvin: 3500, Brightness: math.MaxUint16}
		session.inbound <- protocol.NewMessage(&packets.LightState{Color: color, Power: math.MaxUint16})
		time.Sleep(10 * time.Millisecond)
		deviceSnapshot = session.DeviceSnapshot()
		assert.Equal(t, device.NewColor(color), deviceSnapshot.Color)
		assert.True(t, deviceSnapshot.PoweredOn)

		// Updates product info
		session.inbound <- protocol.NewMessage(&packets.DeviceStateVersion{Product: 55})
		time.Sleep(10 * time.Millisecond)
		deviceSnapshot = session.DeviceSnapshot()
		assert.Equal(t, 55, int(deviceSnapshot.ProductID))
		assert.Equal(t, "LIFX Tile", deviceSnapshot.RegistryName)
		assert.Equal(t, device.LightTypeMatrix, deviceSnapshot.LightType)

		// Updates firmware version
		session.inbound <- protocol.NewMessage(&packets.DeviceStateHostFirmware{VersionMajor: 3, VersionMinor: 50})
		time.Sleep(10 * time.Millisecond)
		assert.Equal(t, "3.50", session.DeviceSnapshot().FirmwareVersion)

		// Updates location
		session.inbound <- protocol.NewMessage(&packets.DeviceStateLocation{Label: [32]byte{'H', 'o', 'm', 'e'}})
		time.Sleep(10 * time.Millisecond)
		assert.Equal(t, "Home", session.DeviceSnapshot().Location)

		// Updates group
		session.inbound <- protocol.NewMessage(&packets.DeviceStateGroup{Label: [32]byte{'B', 'e', 'd', 'r', 'o', 'o', 'm'}})
		time.Sleep(10 * time.Millisecond)
		assert.Equal(t, "Bedroom", session.DeviceSnapshot().Group)

		// Updates matrix properties
		tileDevices := [16]packets.TileStateDevice{{Width: 8, Height: 8}, {Width: 8, Height: 8}}
		session.inbound <- protocol.NewMessage(&packets.TileStateDeviceChain{TileDevicesCount: 2, TileDevices: tileDevices})
		time.Sleep(10 * time.Millisecond)
		assert.Equal(t, int(8), session.DeviceSnapshot().MatrixProperties.Height)
		assert.Equal(t, int(8), session.DeviceSnapshot().MatrixProperties.Width)
		assert.Equal(t, int(2), session.DeviceSnapshot().MatrixProperties.ChainLength)

		// Updates LastSeeenAt
		nowBeforeUpdate := time.Now()
		session.inbound <- protocol.NewMessage(&packets.DeviceStateUnhandled{})
		time.Sleep(10 * time.Millisecond)
		assert.Greater(t, session.DeviceSnapshot().LastSeenAt, nowBeforeUpdate)

		session.Close()
	})
}

func Test_preflightHandshake(t *testing.T) {
	var (
		addr0   = &net.UDPAddr{IP: net.IPv4(192, 168, 0, 10)}
		serial0 = device.Serial([8]byte{1, 0, 0, 0, 0, 0, 0, 0})

		cfg0 = &Config{
			discoveryPeriod:                 defaultDiscoveryPeriod,
			highFrequencyStateRefreshPeriod: defaulthighFrequencyStateRefreshPeriod,
			lowFrequencyStateRefreshPeriod:  defaultlowFrequencyStateRefreshPeriod,
		}
	)

	testCases := map[string]struct {
		msgs        []*protocol.Message
		wantDevice  *device.Device
		wantTimeout bool
	}{
		"single zone": {
			msgs: []*protocol.Message{
				protocol.NewMessage(&packets.DeviceStateLabel{Label: [32]byte{'S', 'Z'}}),
				protocol.NewMessage(&packets.DeviceStateVersion{Product: 225}),
				protocol.NewMessage(&packets.DeviceStateHostFirmware{VersionMajor: 3, VersionMinor: 90}),
				protocol.NewMessage(&packets.DeviceStateLocation{Label: [32]byte{'L'}}),
				protocol.NewMessage(&packets.DeviceStateGroup{Label: [32]byte{'G'}}),
			},
			wantDevice: &device.Device{
				Address: addr0, Serial: serial0,
				Label: "SZ", ProductID: 225, FirmwareVersion: "3.90",
				LightType: device.LightTypeSingleZone, Location: "L", Group: "G",
			},
		},
		"multizone": {
			msgs: []*protocol.Message{
				protocol.NewMessage(&packets.DeviceStateLabel{Label: [32]byte{'M', 'Z'}}),
				protocol.NewMessage(&packets.DeviceStateVersion{Product: 214}),
				protocol.NewMessage(&packets.DeviceStateHostFirmware{VersionMajor: 3, VersionMinor: 90}),
				protocol.NewMessage(&packets.DeviceStateLocation{Label: [32]byte{'L'}}),
				protocol.NewMessage(&packets.DeviceStateGroup{Label: [32]byte{'G'}}),
			},
			wantDevice: &device.Device{
				Address: addr0, Serial: serial0,
				Label: "MZ", ProductID: 214, FirmwareVersion: "3.90",
				LightType: device.LightTypeMultiZone, Location: "L", Group: "G",
			},
		},
		"matrix": {
			msgs: []*protocol.Message{
				protocol.NewMessage(&packets.DeviceStateLabel{Label: [32]byte{'M', 'X'}}),
				protocol.NewMessage(&packets.DeviceStateVersion{Product: 201}),
				protocol.NewMessage(&packets.DeviceStateHostFirmware{VersionMajor: 3, VersionMinor: 90}),
				protocol.NewMessage(&packets.DeviceStateLocation{Label: [32]byte{'L'}}),
				protocol.NewMessage(&packets.DeviceStateGroup{Label: [32]byte{'G'}}),
				protocol.NewMessage(&packets.TileStateDeviceChain{TileDevicesCount: 1, TileDevices: [16]packets.TileStateDevice{{Width: 3}}}),
			},
			wantDevice: &device.Device{
				Address: addr0, Serial: serial0,
				Label: "MX", ProductID: 201, FirmwareVersion: "3.90",
				LightType: device.LightTypeMatrix, Location: "L", Group: "G",
				MatrixProperties: device.MatrixProperties{ChainLength: 1, Width: 3, ChainState: [][64]packets.LightHsbk{{}}},
			},
		},
		"times out with missing fields": {
			msgs: []*protocol.Message{
				protocol.NewMessage(&packets.DeviceStateVersion{Product: 225}),
			},
			wantDevice: &device.Device{
				Address: addr0, Serial: serial0, ProductID: 225, LightType: device.LightTypeSingleZone,
			},
		},
	}

	// Make wait times testable
	preflightHandshakeTimeout := 2 * time.Millisecond
	preflightHandshakeWait := 5 * time.Millisecond

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			mockClient := newMockClient()
			session, err := NewDeviceSession(addr0, serial0, mockClient, cfg0)
			require.NoError(t, err)

			done := make(chan struct{})
			go func() {
				session.preflightHandshake(preflightHandshakeTimeout, preflightHandshakeWait)
				close(done)
			}()

			for _, msg := range tc.msgs {
				session.inbound <- msg
			}

			select {
			case <-done:
			case <-time.After(10 * time.Millisecond):
				t.Fatal("Timed out")
			}

			if diff := cmp.Diff(session.device, tc.wantDevice, cmpopts.IgnoreFields(device.Device{}, "RegistryName", "LastSeenAt")); diff != "" {
				t.Fatal("Got diff in device:\n", diff)
			}
		})
	}
}
