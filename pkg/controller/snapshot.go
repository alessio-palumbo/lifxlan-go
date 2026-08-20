package controller

import (
	"context"
	"fmt"
	"time"

	"github.com/alessio-palumbo/lifxlan-go/pkg/device"
	"github.com/alessio-palumbo/lifxlan-go/pkg/messages"
	"github.com/alessio-palumbo/lifxlan-go/pkg/protocol"
	"github.com/alessio-palumbo/lifxprotocol-go/gen/protocol/packets"
)

const (
	defaultSnapshotTimeout      = 3 * time.Second
	defaultSnapshotPollInterval = 250 * time.Millisecond
	defaultRestoreAttempts      = 1
	defaultRestoreRetryDelay    = 120 * time.Millisecond
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

// RestoreOptions configures state snapshot restore.
type RestoreOptions struct {
	// Duration is the transition duration for color and light power restore.
	Duration time.Duration
	// Attempts is how many times each snapshot device is restored. Zero uses one
	// attempt.
	Attempts int
	// RetryDelay is the delay between restore attempts. Zero uses the default.
	RetryDelay time.Duration
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

// RestoreStateSnapshot restores color and power from snapshot.
func (c *Controller) RestoreStateSnapshot(ctx context.Context, snapshot device.StateSnapshot, opts RestoreOptions) error {
	if ctx == nil {
		ctx = context.Background()
	}
	opts = normalizeRestoreOptions(opts)

	for _, state := range snapshot.Devices {
		if err := c.restoreDeviceState(ctx, state, opts); err != nil {
			return err
		}
	}
	return nil
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

func normalizeRestoreOptions(opts RestoreOptions) RestoreOptions {
	if opts.Attempts <= 0 {
		opts.Attempts = defaultRestoreAttempts
	}
	if opts.RetryDelay <= 0 {
		opts.RetryDelay = defaultRestoreRetryDelay
	}
	return opts
}

func (c *Controller) restoreDeviceState(ctx context.Context, state device.DeviceStateSnapshot, opts RestoreOptions) error {
	for attempt := 0; attempt < opts.Attempts; attempt++ {
		if err := ctx.Err(); err != nil {
			return err
		}
		if err := c.restoreDeviceStateOnce(state, opts.Duration); err != nil {
			return err
		}
		if attempt == opts.Attempts-1 {
			break
		}
		timer := time.NewTimer(opts.RetryDelay)
		select {
		case <-ctx.Done():
			timer.Stop()
			return ctx.Err()
		case <-timer.C:
		}
	}
	return nil
}

func (c *Controller) restoreDeviceStateOnce(state device.DeviceStateSnapshot, duration time.Duration) error {
	if err := c.restoreSnapshotColors(state, duration); err != nil {
		return err
	}
	return c.restoreSnapshotPower(state, duration)
}

func (c *Controller) restoreSnapshotColors(state device.DeviceStateSnapshot, duration time.Duration) error {
	switch state.LightType {
	case device.LightTypeMatrix:
		if len(state.MatrixChains) > 0 {
			return c.restoreMatrixSnapshotColors(state.Serial, state.MatrixWidth, state.MatrixChains, duration)
		}
	case device.LightTypeMultiZone:
		if len(state.Zones) > 0 {
			return c.sendSnapshotMessages(state.Serial, messages.SetMultizoneExtendedColors(0, state.Zones, duration)...)
		}
	}

	hue := state.Color.Hue
	saturation := state.Color.Saturation
	brightness := state.Color.Brightness
	kelvin := state.Color.Kelvin
	return c.Send(state.Serial, messages.SetColor(&hue, &saturation, &brightness, &kelvin, duration, 0))
}

func (c *Controller) restoreMatrixSnapshotColors(serial device.Serial, width int, chains [][]packets.LightHsbk, duration time.Duration) error {
	for chainIndex, colors := range chains {
		if len(colors) == 0 {
			continue
		}
		sendWidth := matrixRestoreWidth(width, len(colors))
		if err := c.sendSnapshotMessages(serial, messages.SetMatrixColorsFromSlice(chainIndex, 1, sendWidth, colors, duration)...); err != nil {
			return err
		}
	}
	return nil
}

func (c *Controller) restoreSnapshotPower(state device.DeviceStateSnapshot, duration time.Duration) error {
	if state.PoweredOn {
		if duration > 0 {
			return c.Send(state.Serial, messages.SetPowerOn(duration))
		}
		return c.Send(state.Serial, messages.SetPowerOn())
	}
	if duration > 0 {
		return c.Send(state.Serial, messages.SetPowerOff(duration))
	}
	return c.Send(state.Serial, messages.SetPowerOff())
}

func (c *Controller) sendSnapshotMessages(serial device.Serial, msgs ...*protocol.Message) error {
	for _, msg := range msgs {
		if err := c.Send(serial, msg); err != nil {
			return err
		}
	}
	return nil
}

func matrixRestoreWidth(width, colorCount int) int {
	if width > 0 {
		return width
	}
	if colorCount == 64 {
		return 8
	}
	return max(colorCount, 1)
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
