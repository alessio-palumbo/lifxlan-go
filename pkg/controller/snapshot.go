package controller

import (
	"context"
	"fmt"
	"time"

	"github.com/alessio-palumbo/lifxlan-go/pkg/device"
)

const (
	defaultSnapshotTimeout      = 3 * time.Second
	defaultSnapshotPollInterval = 250 * time.Millisecond
)

// SnapshotOptions configures state snapshot capture.
type SnapshotOptions struct {
	// Timeout is the maximum time to wait for the controller cache to contain
	// restorable state. Zero uses the default timeout.
	Timeout time.Duration
	// PollInterval is how often missing restorable state is requested. Zero uses
	// the default poll interval.
	PollInterval time.Duration
}

// CaptureStateSnapshot requests and captures restorable cached state for serials.
//
// Target selection is intentionally left to callers. Use device selector helpers,
// Controller.GetDevices, or application-specific state to choose serials.
func (c *Controller) CaptureStateSnapshot(ctx context.Context, serials []device.Serial, opts SnapshotOptions) (device.StateSnapshot, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	if len(serials) == 0 {
		return device.StateSnapshot{}, fmt.Errorf("no snapshot serials provided")
	}

	opts = normalizeSnapshotOptions(opts)
	selected := uniqueSerials(serials)
	selectedSet := serialSet(selected)
	deadline := time.NewTimer(opts.Timeout)
	defer deadline.Stop()

	for {
		devices := c.devicesForSnapshot(selected, selectedSet)
		if err := c.requestRestorableState(devices); err != nil {
			return device.StateSnapshot{}, err
		}
		if snapshotReady(selected, devices) {
			return device.NewStateSnapshot(devices), nil
		}

		poll := time.NewTimer(opts.PollInterval)
		select {
		case <-ctx.Done():
			poll.Stop()
			return device.StateSnapshot{}, ctx.Err()
		case <-deadline.C:
			poll.Stop()
			return device.StateSnapshot{}, fmt.Errorf("timed out capturing restorable state")
		case <-poll.C:
		}
	}
}

func normalizeSnapshotOptions(opts SnapshotOptions) SnapshotOptions {
	if opts.Timeout <= 0 {
		opts.Timeout = defaultSnapshotTimeout
	}
	if opts.PollInterval <= 0 {
		opts.PollInterval = defaultSnapshotPollInterval
	}
	return opts
}

func uniqueSerials(serials []device.Serial) []device.Serial {
	seen := make(map[device.Serial]bool, len(serials))
	out := make([]device.Serial, 0, len(serials))
	for _, serial := range serials {
		if seen[serial] {
			continue
		}
		seen[serial] = true
		out = append(out, serial)
	}
	return out
}

func serialSet(serials []device.Serial) map[device.Serial]bool {
	selected := make(map[device.Serial]bool, len(serials))
	for _, serial := range serials {
		selected[serial] = true
	}
	return selected
}

func (c *Controller) devicesForSnapshot(selected []device.Serial, selectedSet map[device.Serial]bool) []device.Device {
	devices := c.GetDevices()
	bySerial := make(map[device.Serial]device.Device, len(selectedSet))
	for _, d := range devices {
		if selectedSet[d.Serial] {
			bySerial[d.Serial] = d
		}
	}

	out := make([]device.Device, 0, len(selected))
	for _, serial := range selected {
		if d, ok := bySerial[serial]; ok {
			out = append(out, d)
		}
	}
	return out
}

func (c *Controller) requestRestorableState(devices []device.Device) error {
	for _, d := range devices {
		for _, msg := range device.RestorableStateMessages(d) {
			if err := c.Send(d.Serial, msg); err != nil {
				return fmt.Errorf("request restorable state from %s: %w", d.Serial, err)
			}
		}
	}
	return nil
}

func snapshotReady(selected []device.Serial, devices []device.Device) bool {
	if len(devices) != len(selected) {
		return false
	}
	for _, d := range devices {
		if !device.RestorableStateReady(d) {
			return false
		}
	}
	return true
}
