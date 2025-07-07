package controller

import (
	"math"
	"net"
	"testing"
	"time"

	"github.com/alessio-palumbo/lifxlan-go/pkg/protocol"
	"github.com/alessio-palumbo/lifxprotocol-go/gen/protocol/packets"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSession(t *testing.T) {
	var (
		addr0   = &net.UDPAddr{IP: net.IPv4(192, 168, 0, 10)}
		serial0 = Serial([8]byte{1, 0, 0, 0, 0, 0, 0, 0})

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
		for _, p := range DeviceStateMessages() {
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

		gotMsgs := make(map[uint16]int)
		timeout := time.After(10 * time.Millisecond)
	outer:
		for {
			select {
			case msg := <-mockClient.sends:
				switch msg.Payload.PayloadType() {
				case uint16(packets.PayloadTypeDeviceGetLabel), uint16(packets.PayloadTypeDeviceGetHostFirmware),
					uint16(packets.PayloadTypeDeviceGetLocation), uint16(packets.PayloadTypeDeviceGetGroup):
					gotMsgs[msg.Payload.PayloadType()]++
				}
			case <-timeout:
				break outer
			}
		}

		for _, n := range gotMsgs {
			assert.Greater(t, n, 5)
		}
		session.Close()
	})

	t.Run("Updates state", func(t *testing.T) {
		mockClient := newMockClient()
		session, err := NewDeviceSession(addr0, serial0, mockClient, cfg0)
		require.NoError(t, err)

		wantDevice := Device{
			Serial: Serial(serial0), Address: addr0,
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
		assert.Equal(t, NewColor(color), deviceSnapshot.Color)
		assert.True(t, deviceSnapshot.PoweredOn)

		// Updates product info
		session.inbound <- protocol.NewMessage(&packets.DeviceStateVersion{Product: 55})
		time.Sleep(10 * time.Millisecond)
		deviceSnapshot = session.DeviceSnapshot()
		assert.Equal(t, 55, int(deviceSnapshot.ProductID))
		assert.Equal(t, "LIFX Tile", deviceSnapshot.RegistryName)
		assert.Equal(t, LightTypeMatrix, deviceSnapshot.LightType)

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

		// Updates LastSeeenAt
		nowBeforeUpdate := time.Now()
		session.inbound <- protocol.NewMessage(&packets.DeviceStateUnhandled{})
		time.Sleep(10 * time.Millisecond)
		assert.Greater(t, session.DeviceSnapshot().LastSeenAt, nowBeforeUpdate)

		session.Close()
	})
}
