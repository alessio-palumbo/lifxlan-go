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

func TestControllerPing(t *testing.T) {
	sender := newMockClient()
	serial := device.Serial{1, 2, 3}
	session := &deviceSession{
		sender:  sender,
		logger:  discardLogger(),
		device:  device.NewDevice(&net.UDPAddr{IP: net.IPv4(192, 168, 1, 2)}, serial),
		inbound: make(chan *protocol.Message, 2),
		done:    make(chan struct{}),
	}
	go session.recvloop()
	defer session.close()

	ctrl := &Controller{sessions: map[device.Serial]*deviceSession{serial: session}}
	result := make(chan struct {
		rtt time.Duration
		err error
	}, 1)
	go func() {
		rtt, err := ctrl.Ping(context.Background(), serial)
		result <- struct {
			rtt time.Duration
			err error
		}{rtt: rtt, err: err}
	}()

	request := <-sender.sends
	echo, ok := request.Payload.(*packets.DeviceEchoRequest)
	if !ok {
		t.Fatalf("payload = %T, want *packets.DeviceEchoRequest", request.Payload)
	}
	if request.Target() != [8]byte(serial) {
		t.Fatalf("target = %x, want %x", request.Target(), serial)
	}
	unmatched := echo.Payload
	unmatched[0]++
	session.inbound <- protocol.NewMessage(&packets.DeviceEchoResponse{Payload: unmatched})
	deadline := time.Now().Add(time.Second)
	for session.deviceSnapshot().LastSeenAt.IsZero() && time.Now().Before(deadline) {
		time.Sleep(time.Millisecond)
	}
	if session.deviceSnapshot().LastSeenAt.IsZero() {
		t.Fatal("unmatched response was not processed")
	}
	select {
	case got := <-result:
		t.Fatalf("unmatched response completed ping: %+v", got)
	default:
	}
	session.inbound <- protocol.NewMessage(&packets.DeviceEchoResponse{Payload: echo.Payload})

	got := <-result
	if got.err != nil {
		t.Fatal(got.err)
	}
	if got.rtt <= 0 {
		t.Fatalf("round-trip time = %v, want positive duration", got.rtt)
	}
}

func TestControllerPingErrors(t *testing.T) {
	serial := device.Serial{1}

	t.Run("device not found", func(t *testing.T) {
		ctrl := &Controller{sessions: make(map[device.Serial]*deviceSession)}
		_, err := ctrl.Ping(context.Background(), serial)
		if !errors.Is(err, ErrDeviceNotFound) {
			t.Fatalf("error = %v, want ErrDeviceNotFound", err)
		}
	})

	t.Run("context canceled before send", func(t *testing.T) {
		sender := newMockClient()
		session := pingTestSession(sender, serial)
		ctx, cancel := context.WithCancel(context.Background())
		cancel()

		ctrl := &Controller{sessions: map[device.Serial]*deviceSession{serial: session}}
		_, err := ctrl.Ping(ctx, serial)
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("error = %v, want context.Canceled", err)
		}
		if len(sender.sends) != 0 {
			t.Fatal("canceled ping sent a packet")
		}
	})

	t.Run("context deadline", func(t *testing.T) {
		sender := newMockClient()
		session := pingTestSession(sender, serial)
		ctx, cancel := context.WithTimeout(context.Background(), time.Millisecond)
		defer cancel()

		ctrl := &Controller{sessions: map[device.Serial]*deviceSession{serial: session}}
		_, err := ctrl.Ping(ctx, serial)
		if !errors.Is(err, context.DeadlineExceeded) {
			t.Fatalf("error = %v, want context.DeadlineExceeded", err)
		}
		session.pingMu.Lock()
		pending := len(session.pendingPings)
		session.pingMu.Unlock()
		if pending != 0 {
			t.Fatalf("pending pings = %d, want 0", pending)
		}
	})

	t.Run("session closes", func(t *testing.T) {
		sender := newMockClient()
		session := pingTestSession(sender, serial)
		ctrl := &Controller{sessions: map[device.Serial]*deviceSession{serial: session}}
		result := make(chan error, 1)
		go func() {
			_, err := ctrl.Ping(context.Background(), serial)
			result <- err
		}()
		<-sender.sends
		session.close()
		if err := <-result; !errors.Is(err, ErrSessionClosed) {
			t.Fatalf("error = %v, want ErrSessionClosed", err)
		}
	})

	t.Run("session already closed", func(t *testing.T) {
		sender := newMockClient()
		session := pingTestSession(sender, serial)
		session.close()
		ctrl := &Controller{sessions: map[device.Serial]*deviceSession{serial: session}}
		_, err := ctrl.Ping(context.Background(), serial)
		if !errors.Is(err, ErrSessionClosed) {
			t.Fatalf("error = %v, want ErrSessionClosed", err)
		}
		if len(sender.sends) != 0 {
			t.Fatal("closed session sent a ping packet")
		}
	})
}

func pingTestSession(sender sender, serial device.Serial) *deviceSession {
	return &deviceSession{
		sender: sender,
		device: device.NewDevice(&net.UDPAddr{IP: net.IPv4(192, 168, 1, 2)}, serial),
		done:   make(chan struct{}),
	}
}
