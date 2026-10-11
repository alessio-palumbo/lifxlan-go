package controller

import (
	"net"
	"time"

	"github.com/alessio-palumbo/lifxlan-go/pkg/protocol"
)

func cloneEndpoint(addr *net.UDPAddr) *net.UDPAddr {
	if addr == nil {
		return nil
	}
	copy := *addr
	copy.IP = append(net.IP(nil), addr.IP...)
	return &copy
}

func serviceEndpoint(from *net.UDPAddr, port uint32) (*net.UDPAddr, bool) {
	if from == nil || from.IP.To16() == nil || from.IP.IsUnspecified() || from.IP.IsMulticast() || from.IP.Equal(net.IPv4bcast) || port == 0 || port > 65535 {
		return nil, false
	}
	endpoint := cloneEndpoint(from)
	endpoint.Port = int(port)
	return endpoint, true
}

func (s *deviceSession) updateEndpoint(addr *net.UDPAddr) bool {
	s.sendMu.Lock()
	defer s.sendMu.Unlock()
	s.mu.Lock()
	defer s.mu.Unlock()
	old := s.device.Address
	if old != nil && old.IP.Equal(addr.IP) && old.Port == addr.Port && old.Zone == addr.Zone {
		return false
	}
	s.device.Address = cloneEndpoint(addr)
	s.endpointEpoch++
	if s.endpointChanged != nil {
		close(s.endpointChanged)
	}
	s.endpointChanged = make(chan struct{})
	// Keep monotonic stamps so captures started before migration can require
	// new observations without waiting for a reset counter to catch up.
	s.observations = snapshotObservations{generation: s.observations.generation + 1, product: s.observations.product}
	now := time.Now()
	s.device.LastSeenAt, s.device.LastUpdatedAt = now, now
	return true
}

func (s *deviceSession) deliverFromEndpoint(msg *protocol.Message, from *net.UDPAddr) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	if from == nil || s.device.Address == nil || !from.IP.Equal(s.device.Address.IP) || from.Zone != s.device.Address.Zone {
		return true
	} // stale peer, intentionally ignored
	if s.inboundEpochs == nil {
		s.inboundEpochs = make(map[*protocol.Message]uint64)
	}
	s.inboundEpochs[msg] = s.endpointEpoch
	select {
	case s.inbound <- msg:
		return true
	default:
		delete(s.inboundEpochs, msg)
		return false
	}
}
