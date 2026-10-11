package controller

import (
	"context"
	"errors"
	"net"
	"sync"
	"testing"
	"time"

	"github.com/alessio-palumbo/lifxlan-go/pkg/device"
	"github.com/alessio-palumbo/lifxlan-go/pkg/protocol"
	"github.com/alessio-palumbo/lifxprotocol-go/gen/protocol/enums"
	"github.com/alessio-palumbo/lifxprotocol-go/gen/protocol/packets"
	"github.com/stretchr/testify/require"
)

type endpointClient struct {
	*mockClient
	mu        sync.Mutex
	addresses []*net.UDPAddr
	types     []uint16
}

func (c *endpointClient) Send(addr *net.UDPAddr, msg *protocol.Message) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.addresses = append(c.addresses, cloneEndpoint(addr))
	c.types = append(c.types, msg.Type())
	return nil
}

func TestEndpointMigrationKeepsSessionAndRoutesSends(t *testing.T) {
	c := &endpointClient{mockClient: newMockClient()}
	ctrl, err := New(WithClient(c))
	require.NoError(t, err)
	defer ctrl.Close()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	events := ctrl.SubscribeDevices(ctx)
	<-events // initial inventory control event
	serial := device.Serial{1}
	announce := func(ip string, port uint32) {
		m := protocol.NewMessage(&packets.DeviceStateService{Service: enums.DeviceServiceDEVICESERVICEUDP, Port: port})
		m.SetTarget(serial)
		c.inbound <- recvMsg{addr: &net.UDPAddr{IP: net.ParseIP(ip), Port: 9999}, msg: m}
	}
	announce("172.16.0.1", 56700)
	require.Eventually(t, func() bool { _, ok := ctrl.GetDevice(serial); return ok }, time.Second, time.Millisecond)
	require.Equal(t, DeviceEventAdded, (<-events).Type)
	ctrl.mu.RLock()
	session := ctrl.sessions[serial]
	ctrl.mu.RUnlock()
	session.mu.Lock()
	session.device.Label = "kept"
	session.observations.product = true
	session.observations.generation = 10
	session.observations.light = 10
	session.mu.Unlock()
	announce("192.168.1.90", 56800)
	require.Eventually(t, func() bool {
		d, _ := ctrl.GetDevice(serial)
		return d.Address.Port == 56800 && d.Address.IP.Equal(net.ParseIP("192.168.1.90"))
	}, time.Second, time.Millisecond)
	event := <-events
	require.Equal(t, DeviceEventUpdated, event.Type)
	require.True(t, event.Changes.Has(DeviceChangeAddress))
	ctrl.mu.RLock()
	require.Same(t, session, ctrl.sessions[serial])
	ctrl.mu.RUnlock()
	d, _ := ctrl.GetDevice(serial)
	require.Equal(t, "kept", d.Label)
	session.mu.RLock()
	require.Zero(t, session.observations.light)
	require.Greater(t, session.observations.generation, uint64(10))
	session.mu.RUnlock()
	announce("192.168.1.90", 56800)
	require.NoError(t, ctrl.Send(serial, protocol.NewMessage(&packets.DeviceGetInfo{})))
	c.mu.Lock()
	require.True(t, c.addresses[len(c.addresses)-1].IP.Equal(d.Address.IP))
	require.Equal(t, 56800, c.addresses[len(c.addresses)-1].Port)
	c.mu.Unlock()
	// Non-UDP and invalid port advertisements cannot move the current endpoint.
	announce("192.168.1.91", 0)
	announce("192.168.1.91", 70000)
	label := protocol.NewMessage(&packets.DeviceStateLabel{Label: [32]byte{'o', 'l', 'd'}})
	label.SetTarget(serial)
	c.inbound <- recvMsg{addr: &net.UDPAddr{IP: net.ParseIP("172.16.0.1")}, msg: label}
	fresh := protocol.NewMessage(&packets.LightState{})
	fresh.SetTarget(serial)
	c.inbound <- recvMsg{addr: d.Address, msg: fresh}
	require.Eventually(t, func() bool { session.mu.RLock(); defer session.mu.RUnlock(); return session.observations.light > 10 }, time.Second, time.Millisecond)
	d, _ = ctrl.GetDevice(serial)
	require.Equal(t, "kept", d.Label)
	require.Equal(t, 56800, d.Address.Port)
	require.Empty(t, events, "duplicate advertisements must not emit updates")
}

func TestQueuedOldEndpointReplyAndPendingPing(t *testing.T) {
	c := &endpointClient{mockClient: newMockClient()}
	a := &net.UDPAddr{IP: net.ParseIP("172.16.0.1"), Port: 56700}
	b := &net.UDPAddr{IP: net.ParseIP("192.168.1.90"), Port: 56700}
	s := &deviceSession{sender: c, logger: discardLogger(), device: device.NewDevice(a, device.Serial{1}), inbound: make(chan *protocol.Message, 10), done: make(chan struct{}), endpointChanged: make(chan struct{})}
	s.device.Label = "kept"
	require.True(t, s.deliverFromEndpoint(protocol.NewMessage(&packets.DeviceStateLabel{Label: [32]byte{'o', 'l', 'd'}}), a))
	require.True(t, s.updateEndpoint(b))
	go s.recvloop()
	defer s.close()
	require.Eventually(t, func() bool { s.mu.RLock(); defer s.mu.RUnlock(); return len(s.inboundEpochs) == 0 }, time.Second, time.Millisecond)
	require.Equal(t, "kept", s.deviceSnapshot().Label)
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	result := make(chan error, 1)
	go func() { _, err := s.ping(ctx); result <- err }()
	require.Eventually(t, func() bool {
		c.mu.Lock()
		defer c.mu.Unlock()
		for _, v := range c.types {
			if v == 58 {
				return true
			}
		}
		return false
	}, time.Second, time.Millisecond)
	require.True(t, s.updateEndpoint(a))
	require.True(t, errors.Is(<-result, ErrEndpointChanged))
	// Endpoint snapshots own their IP slices, including concurrent send/read use.
	var wg sync.WaitGroup
	for range 3 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := 0; i < 50; i++ {
				s.deviceSnapshot()
				s.send(protocol.NewMessage(&packets.LightGet{}))
			}
		}()
	}
	for range 50 {
		s.updateEndpoint(b)
		s.updateEndpoint(a)
	}
	wg.Wait()
}
