package controller

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/alessio-palumbo/lifxlan-go/pkg/device"
)

// ErrSessionClosed indicates that a device session ended while an operation
// was waiting for its response.
var ErrSessionClosed = errors.New("device session closed")

// Ping sends one echo request and returns its round-trip time. It reports the
// duration of this request only; callers that need latency statistics should
// take multiple sequential samples. Context cancellation or deadlines stop the
// wait but cannot recall a datagram that has already been sent.
func (c *Controller) Ping(ctx context.Context, serial device.Serial) (time.Duration, error) {
	c.mu.RLock()
	session, ok := c.sessions[serial]
	c.mu.RUnlock()
	if !ok {
		return 0, fmt.Errorf("%w: %s", ErrDeviceNotFound, serial)
	}

	rtt, err := session.ping(ctx)
	if err != nil {
		return 0, fmt.Errorf("ping device %s: %w", serial, err)
	}
	return rtt, nil
}
