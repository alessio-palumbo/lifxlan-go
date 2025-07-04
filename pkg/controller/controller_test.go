package controller

import (
	"net"
	"sync"
	"testing"
	"time"

	"github.com/alessio-palumbo/lifxlan-go/internal/protocol"
	"github.com/alessio-palumbo/lifxlan-go/pkg/client"
	"github.com/alessio-palumbo/lifxprotocol-go/gen/protocol/enums"
	"github.com/alessio-palumbo/lifxprotocol-go/gen/protocol/packets"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestController(t *testing.T) {
	var (
		addr0   = &net.UDPAddr{IP: net.IPv4(192, 168, 0, 10)}
		addr1   = &net.UDPAddr{IP: net.IPv4(192, 168, 0, 11)}
		target0 = [8]byte{1, 0, 0, 0, 0, 0, 0, 0}
		target1 = [8]byte{2, 0, 0, 0, 0, 0, 0, 0}
	)
	t.Run("Sets up default configuration", func(t *testing.T) {
		mockClient := newMockClient()
		ctrl, err := New(WithClient(mockClient))
		require.NoError(t, err)
		defer ctrl.Close()

		assert.Equal(t, defaultDiscoveryPeriod, ctrl.cfg.discoveryPeriod)
		assert.Equal(t, defaulthighFrequencyStateRefreshPeriod, ctrl.cfg.highFrequencyStateRefreshPeriod)
		assert.Equal(t, defaultlowFrequencyStateRefreshPeriod, ctrl.cfg.lowFrequencyStateRefreshPeriod)
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

		payload := &packets.LightGet{}
		msg := protocol.NewMessage(payload)
		err = ctrl.Send(addr0, msg)
		require.NoError(t, err)

		ctrl.Close()
		assert.Equal(t, len(mockClient.sends), 0)
	})

	t.Run("Adds sessions", func(t *testing.T) {
		mockClient := newMockClient()
		ctrl, err := New(WithClient(mockClient))
		require.NoError(t, err)
		defer ctrl.Close()

		err = ctrl.addSession(addr0, target0)
		require.NoError(t, err)
		assert.Equal(t, len(ctrl.sessions), 1)

		s0 := ctrl.sessions[addr0.IP.String()]
		assert.NotNil(t, s0)
		assert.NotNil(t, s0.device)
		assert.Equal(t, Serial(target0), s0.device.Serial)
	})

	t.Run("Sends to an addr with session", func(t *testing.T) {
		mockClient := newMockClient()
		ctrl, err := New(WithClient(mockClient))
		require.NoError(t, err)

		// Do not use NewDeviceSession to prevent runninng state update goroutine
		session := &DeviceSession{sender: mockClient, device: NewDevice(addr0, target0), done: make(chan struct{})}
		ctrl.sessions[addr0.IP.String()] = session

		payload := &packets.LightGet{}
		msg := protocol.NewMessage(payload)

		err = ctrl.Send(addr0, msg)
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

		err = ctrl.addSession(addr0, target0)
		require.NoError(t, err)
		err = ctrl.addSession(addr1, target1)
		require.NoError(t, err)

		devices := ctrl.GetDevices()
		assert.Equal(t, 2, len(devices))
		assert.Equal(t, Serial(target0), devices[0].Serial)
		assert.Equal(t, Serial(target1), devices[1].Serial)

	})

	t.Run("Adds a newly discovered device to sessions", func(t *testing.T) {
		mockClient := newMockClient()
		ctrl, err := New(WithClient(mockClient))
		require.NoError(t, err)
		defer ctrl.Close()

		payload := &packets.DeviceStateService{Service: enums.DeviceServiceDEVICESERVICEUDP}
		msg := protocol.NewMessage(payload)
		msg.SetTarget(target0)

		mockClient.inbound <- recvMsg{msg: msg, addr: addr0}
		time.Sleep(10 * time.Millisecond)
		assert.Equal(t, 1, len(ctrl.GetDevices()))
		assert.Equal(t, Serial(target0), ctrl.GetDevices()[0].Serial)
	})

	t.Run("Routes state messages to device with session", func(t *testing.T) {
		mockClient := newMockClient()
		ctrl, err := New(WithClient(mockClient))
		require.NoError(t, err)
		defer ctrl.Close()

		err = ctrl.addSession(addr0, target0)
		require.NoError(t, err)

		label0 := [32]byte{0, 0, 0, 1}
		payload := &packets.DeviceStateLabel{Label: label0}
		msg := protocol.NewMessage(payload)
		msg.SetTarget(target0)

		mockClient.inbound <- recvMsg{msg: msg, addr: addr0}
		time.Sleep(10 * time.Millisecond)
		assert.Equal(t, 1, len(ctrl.GetDevices()))
		assert.Equal(t, Serial(target0), ctrl.GetDevices()[0].Serial)
	})

	t.Run("Terminate sessions when closed", func(t *testing.T) {
		mockClient := newMockClient()
		ctrl, err := New(WithClient(mockClient))
		require.NoError(t, err)

		session := &DeviceSession{
			sender: mockClient, device: NewDevice(addr0, target0), done: make(chan struct{}),
		}
		ctrl.sessions[addr0.IP.String()] = session

		ctrl.Close()
		select {
		case <-session.done:
		case <-time.After(10 * time.Millisecond):
			t.Fatal("Session channel was not closed")
		}
	})
}

func BenchmarkControllerGetDevices(b *testing.B) {
	var (
		addr0   = &net.UDPAddr{IP: net.IPv4(192, 168, 0, 10)}
		target0 = [8]byte{1, 0, 0, 0, 0, 0, 0, 0}
	)

	mockClient := newMockClient()
	ctrl, err := New(WithClient(mockClient))
	require.NoError(b, err)
	defer ctrl.Close()

	err = ctrl.addSession(addr0, target0)
	require.NoError(b, err)

	msg := protocol.NewMessage(&packets.LightState{})
	done := make(chan struct{})
	go func() {
		ticker := time.NewTicker(10 * time.Millisecond)
		for {
			select {
			case <-done:
				return
			case <-ticker.C:
				mockClient.inbound <- recvMsg{msg: msg, addr: addr0}
			}
		}
	}()

	b.ResetTimer()
	for b.Loop() {
		_ = ctrl.GetDevices()
	}
	close(done)
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
