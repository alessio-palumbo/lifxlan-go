package controller

import (
	"math/rand"
	"net"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/alessio-palumbo/lifxlan-go/pkg/client"
	"github.com/alessio-palumbo/lifxlan-go/pkg/device"
	"github.com/alessio-palumbo/lifxlan-go/pkg/protocol"
	"github.com/alessio-palumbo/lifxprotocol-go/gen/protocol/enums"
	"github.com/alessio-palumbo/lifxprotocol-go/gen/protocol/packets"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestController(t *testing.T) {
	var (
		addr0   = &net.UDPAddr{IP: net.IPv4(192, 168, 0, 10)}
		addr1   = &net.UDPAddr{IP: net.IPv4(192, 168, 0, 11)}
		serial0 = device.Serial([8]byte{1, 0, 0, 0, 0, 0, 0, 0})
		serial1 = device.Serial([8]byte{2, 0, 0, 0, 0, 0, 0, 0})
	)
	t.Run("Sets up default configuration", func(t *testing.T) {
		mockClient := newMockClient()
		ctrl, err := New(WithClient(mockClient))
		require.NoError(t, err)
		defer ctrl.Close()

		assert.Equal(t, defaultDiscoveryPeriod, ctrl.cfg.discoveryPeriod)
		assert.Equal(t, defaultHighFrequencyStateRefreshPeriod, ctrl.cfg.highFrequencyStateRefreshPeriod)
		assert.Equal(t, defaultLowFrequencyStateRefreshPeriod, ctrl.cfg.lowFrequencyStateRefreshPeriod)
		assert.Equal(t, 50*time.Second, ctrl.cfg.deviceLivenessTimeout)
	})

	t.Run("Performs continuous discovery", func(t *testing.T) {
		mockClient := newMockClient()
		ctrl, err := New(WithClient(mockClient), WithDiscoveryPeriod(time.Millisecond))
		require.NoError(t, err)

		time.Sleep(10 * time.Millisecond)
		ctrl.Close()
		assert.Greater(t, len(mockClient.broadcasts), 5)
	})

	t.Run("Skips Send if an addr has no session", func(t *testing.T) {
		mockClient := newMockClient()
		ctrl, err := New(WithClient(mockClient))
		require.NoError(t, err)

		err = ctrl.Send(serial0, protocol.NewMessage(&packets.LightGet{}))
		require.NoError(t, err)

		ctrl.Close()
		assert.Equal(t, len(mockClient.sends), 0)
	})

	t.Run("Adds/Terminates sessions", func(t *testing.T) {
		mockClient := newMockClient()
		ctrl, err := New(WithClient(mockClient))
		require.NoError(t, err)
		defer ctrl.Close()

		ctrl.addSession(addr0, serial0)
		assert.Equal(t, len(ctrl.sessions), 1)

		s0 := ctrl.sessions[serial0]
		assert.NotNil(t, s0)
		assert.NotNil(t, s0.device)
		assert.Equal(t, serial0, s0.device.Serial)

		ctrl.terminateSession(serial0)
		assert.Equal(t, len(ctrl.sessions), 0)
	})

	t.Run("Sends to an addr with session", func(t *testing.T) {
		mockClient := newMockClient()
		ctrl, err := New(WithClient(mockClient))
		require.NoError(t, err)

		// Do not use NewDeviceSession to prevent runninng state update goroutine
		session := &DeviceSession{sender: mockClient, device: device.NewDevice(addr0, serial0), done: make(chan struct{})}
		ctrl.sessions[serial0] = session
		ctrl.wg.Add(1)

		payload := &packets.LightGet{}
		msg := protocol.NewMessage(payload)

		err = ctrl.Send(serial0, protocol.NewMessage(&packets.LightGet{}))
		require.NoError(t, err)
		ctrl.Close()
		assert.Equal(t, 1, len(mockClient.sends))
		recvMsg := <-mockClient.sends
		assert.Equal(t, msg.Payload.PayloadType(), recvMsg.Payload.PayloadType())
	})

	t.Run("Return the current state of devices", func(t *testing.T) {
		mockClient := newMockClient()
		ctrl, err := New(WithClient(mockClient))
		require.NoError(t, err)
		defer ctrl.Close()

		ctrl.addSession(addr0, serial0)
		ctrl.addSession(addr1, serial1)

		devices := ctrl.GetDevices()
		assert.Equal(t, 2, len(devices))
		assert.Equal(t, serial0, devices[0].Serial)
		assert.Equal(t, serial1, devices[1].Serial)

	})

	t.Run("Adds a newly discovered device to sessions", func(t *testing.T) {
		mockClient := newMockClient()
		ctrl, err := New(WithClient(mockClient))
		require.NoError(t, err)
		defer ctrl.Close()

		payload := &packets.DeviceStateService{Service: enums.DeviceServiceDEVICESERVICEUDP}
		msg := protocol.NewMessage(payload)
		msg.SetTarget(serial0)

		mockClient.inbound <- recvMsg{msg: msg, addr: addr0}
		time.Sleep(10 * time.Millisecond)
		assert.Equal(t, 1, len(ctrl.GetDevices()))
		assert.Equal(t, serial0, ctrl.GetDevices()[0].Serial)
	})

	t.Run("Routes state messages to device with session", func(t *testing.T) {
		mockClient := newMockClient()
		ctrl, err := New(WithClient(mockClient))
		require.NoError(t, err)
		defer ctrl.Close()

		ctrl.addSession(addr0, serial0)

		label0 := [32]byte{0, 0, 0, 1}
		payload := &packets.DeviceStateLabel{Label: label0}
		msg := protocol.NewMessage(payload)
		msg.SetTarget(serial0)

		mockClient.inbound <- recvMsg{msg: msg, addr: addr0}
		time.Sleep(10 * time.Millisecond)
		assert.Equal(t, 1, len(ctrl.GetDevices()))
		assert.Equal(t, serial0, ctrl.GetDevices()[0].Serial)
	})

	t.Run("Terminate sessions when closed", func(t *testing.T) {
		mockClient := newMockClient()
		ctrl, err := New(WithClient(mockClient))
		require.NoError(t, err)

		session := &DeviceSession{
			sender: mockClient, device: device.NewDevice(addr0, serial0), done: make(chan struct{}),
		}
		ctrl.sessions[serial0] = session
		ctrl.wg.Add(1)

		ctrl.Close()
		select {
		case <-session.done:
		case <-time.After(10 * time.Millisecond):
			t.Fatal("Session channel was not closed")
		}
	})
}

func BenchmarkControllerGetDevices(b *testing.B) {
	os.Setenv("LIFXLAN_LOG_LEVEL", "error")
	mockClient := newMockClient()
	ctrl, err := New(WithClient(mockClient))
	require.NoError(b, err)
	defer ctrl.Close()

	// Base address
	ipBase := [4]byte{192, 168, 1, 100}
	port := 56700

	for i := range 100 {
		// Increment the last byte of IP
		addr := &net.UDPAddr{
			IP:   net.IPv4(ipBase[0], ipBase[1], ipBase[2], ipBase[3]+byte(i)),
			Port: port,
		}

		// Build serial: top 6 bytes from counter, last 2 = 0
		s := uint64(i + 1)
		serial := [8]byte{
			byte(s >> 40 & 0xFF),
			byte(s >> 32 & 0xFF),
			byte(s >> 24 & 0xFF),
			byte(s >> 16 & 0xFF),
			byte(s >> 8 & 0xFF),
			byte(s >> 0 & 0xFF),
			0,
			0,
		}

		ctrl.addSession(addr, serial)
		ctrl.sessions[serial].device.Label = randomLabel()
	}

	b.ResetTimer()
	for b.Loop() {
		_ = ctrl.GetDevices()
	}
}

// randomLabel returns a random string of 8–10 alphabetic characters.
func randomLabel() string {
	n := 8 + rand.Intn(3) // 8, 9 or 10
	letters := []rune("abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ")
	b := make([]rune, n)
	for i := range b {
		b[i] = letters[rand.Intn(len(letters))]
	}
	return string(b)
}

type mockClient struct {
	sends      chan *protocol.Message
	broadcasts chan struct{}
	inbound    chan recvMsg
	once       sync.Once
	done       chan struct{}
}

type recvMsg struct {
	addr *net.UDPAddr
	msg  *protocol.Message
}

func newMockClient() *mockClient {
	return &mockClient{
		sends:      make(chan *protocol.Message, 100),
		broadcasts: make(chan struct{}, 100),
		inbound:    make(chan recvMsg, 10),
		done:       make(chan struct{}),
	}
}

func (m *mockClient) Send(dst *net.UDPAddr, msg *protocol.Message) error {
	m.sends <- msg
	return nil
}

func (m *mockClient) SendBroadcast(msg *protocol.Message) error {
	m.broadcasts <- struct{}{}
	return nil
}

func (m *mockClient) Receive(timeout time.Duration, recvOne bool, handler client.HandlerFunc) error {
	for {
		select {
		case recvd := <-m.inbound:
			handler(recvd.msg, recvd.addr)
		case <-m.done:
			return nil
		}
	}
}

func (m *mockClient) SetConnDeadline(t time.Time) error {
	m.once.Do(func() {
		close(m.done)
	})
	return nil
}

func (m *mockClient) Close() error {
	return nil
}
