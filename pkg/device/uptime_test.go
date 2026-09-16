package device

import (
	"testing"
	"time"
)

func TestDeviceUptime(t *testing.T) {
	t.Run("unknown", func(t *testing.T) {
		if uptime, ok := (Device{}).Uptime(); ok || uptime != 0 {
			t.Fatalf("Uptime() = (%v, %v), want (0, false)", uptime, ok)
		}
	})

	t.Run("estimated from boot time", func(t *testing.T) {
		want := 2 * time.Hour
		d := Device{EstimatedBootedAt: time.Now().Add(-want)}
		uptime, ok := d.Uptime()
		if !ok {
			t.Fatal("Uptime() reported an unknown baseline")
		}
		if uptime < want || uptime > want+time.Second {
			t.Fatalf("Uptime() = %v, want between %v and %v", uptime, want, want+time.Second)
		}
	})

	t.Run("future estimate is clamped", func(t *testing.T) {
		d := Device{EstimatedBootedAt: time.Now().Add(time.Hour)}
		if uptime, ok := d.Uptime(); !ok || uptime != 0 {
			t.Fatalf("Uptime() = (%v, %v), want (0, true)", uptime, ok)
		}
	})
}
