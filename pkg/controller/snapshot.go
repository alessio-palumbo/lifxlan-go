package controller

import (
	"context"
	"errors"
	"fmt"
	"strings"
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
	// RequireFresh waits for light state and all zone/pixel ranges to be observed
	// after capture starts. By default, complete previously observed state is
	// accepted. Observations are not correlated to individual requests and do
	// not form an atomic snapshot across packets or devices.
	RequireFresh bool
}

// RestoreOptions configures state snapshot restore.
type RestoreOptions struct {
	// Duration is the transition duration for color and light power restore.
	Duration time.Duration
	// Attempts is how many unconditional restore rounds are sent to every
	// snapshot device. Rounds do not wait for acknowledgements or verify applied
	// state. Zero uses one attempt.
	Attempts int
	// RetryDelay is the delay between restore rounds. Zero uses the default.
	RetryDelay time.Duration
}

// CaptureStateSnapshot requests and captures complete observed light state for
// serials. It never treats allocated zone buffers as received state. Powered-off
// matrices require the same pixel coverage as powered-on matrices.
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
	deadline := time.NewTimer(opts.Timeout)
	defer deadline.Stop()
	checks := time.NewTicker(min(opts.PollInterval, 10*time.Millisecond))
	defer checks.Stop()
	baselines := make(map[device.Serial]snapshotBaseline, len(selected))
	var nextRequest time.Time

	for {
		if err := ctx.Err(); err != nil {
			return device.StateSnapshot{}, err
		}
		requestDue := !time.Now().Before(nextRequest)
		snapshot, missing, requests := c.snapshotProgress(selected, baselines, opts.RequireFresh, requestDue)
		for _, request := range requests {
			if err := ctx.Err(); err != nil {
				return device.StateSnapshot{}, err
			}
			if err := request.session.send(request.messages...); err != nil {
				return device.StateSnapshot{}, fmt.Errorf("request restorable state from %s: %w", request.serial, err)
			}
		}
		if len(missing) == 0 {
			return snapshot, nil
		}
		if requestDue {
			nextRequest = time.Now().Add(opts.PollInterval)
		}

		select {
		case <-ctx.Done():
			return device.StateSnapshot{}, ctx.Err()
		case <-deadline.C:
			return device.StateSnapshot{}, fmt.Errorf("timed out capturing restorable state: %s", strings.Join(missing, "; "))
		case <-checks.C:
		}
	}
}

// RestoreStateSnapshot restores color and power from snapshot. Attempts run in
// rounds across all devices, and send failures are joined after every device
// has had an opportunity to restore.
func (c *Controller) RestoreStateSnapshot(ctx context.Context, snapshot device.StateSnapshot, opts RestoreOptions) error {
	if ctx == nil {
		ctx = context.Background()
	}
	if len(snapshot.Devices) == 0 {
		return nil
	}
	opts = normalizeRestoreOptions(opts)

	var restoreErrs []error
	for attempt := range opts.Attempts {
		for _, state := range snapshot.Devices {
			if err := ctx.Err(); err != nil {
				return errors.Join(append(restoreErrs, err)...)
			}
			if err := c.restoreDeviceStateOnce(state, opts.Duration); err != nil {
				restoreErrs = append(restoreErrs, fmt.Errorf(
					"restore device %s (attempt %d/%d): %w",
					state.Serial, attempt+1, opts.Attempts, err,
				))
			}
		}

		if attempt == opts.Attempts-1 {
			break
		}
		timer := time.NewTimer(opts.RetryDelay)
		select {
		case <-ctx.Done():
			timer.Stop()
			return errors.Join(append(restoreErrs, ctx.Err())...)
		case <-timer.C:
		}
	}
	return errors.Join(restoreErrs...)
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

func (c *Controller) restoreDeviceStateOnce(state device.DeviceStateSnapshot, duration time.Duration) error {
	if !state.PoweredOn {
		if err := c.restoreSnapshotPower(state, duration); err != nil {
			return err
		}
		return c.restoreSnapshotColors(state, duration)
	}

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

type snapshotBaseline struct {
	session *deviceSession
	stamp   uint64
}

type snapshotRequest struct {
	serial   device.Serial
	session  *deviceSession
	messages []*protocol.Message
}

// snapshotProgress checks receipt metadata and copies colors under the same
// session lock. Session identity scopes freshness baselines to reconnects.
func (c *Controller) snapshotProgress(selected []device.Serial, baselines map[device.Serial]snapshotBaseline, fresh, requestDue bool) (device.StateSnapshot, []string, []snapshotRequest) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	var missing []string
	var requests []snapshotRequest
	for _, serial := range selected {
		session, ok := c.sessions[serial]
		if !ok {
			missing = append(missing, fmt.Sprintf("%s: device unavailable", serial))
			continue
		}
		session.mu.RLock()
		baseline, known := baselines[serial]
		first := !known || baseline.session != session
		if first {
			baseline = snapshotBaseline{session: session}
			if fresh {
				baseline.stamp = session.observations.generation
			}
			baselines[serial] = baseline
		}
		reason := session.observations.missing(session.device, baseline.stamp)
		if reason != "" {
			missing = append(missing, fmt.Sprintf("%s: %s", serial, reason))
		}
		if first || requestDue && reason != "" {
			msgs := device.RestorableStateMessages(*session.device)
			if !session.observations.product {
				msgs = append(msgs, protocol.NewMessage(&packets.DeviceGetVersion{}))
			}
			requests = append(requests, snapshotRequest{serial: serial, session: session, messages: msgs})
		}
		session.mu.RUnlock()
	}
	if len(missing) != 0 {
		return device.StateSnapshot{}, missing, requests
	}
	// Avoid repeatedly cloning ready devices while another target is pending.
	// Recheck under the lock used for copying in case geometry changed meanwhile.
	snapshot := device.StateSnapshot{Devices: make([]device.DeviceStateSnapshot, 0, len(selected))}
	for _, serial := range selected {
		session := c.sessions[serial]
		session.mu.RLock()
		if reason := session.observations.missing(session.device, baselines[serial].stamp); reason != "" {
			session.mu.RUnlock()
			return device.StateSnapshot{}, []string{fmt.Sprintf("%s: %s", serial, reason)}, requests
		}
		snapshot.Devices = append(snapshot.Devices, device.NewDeviceStateSnapshot(*session.device))
		session.mu.RUnlock()
	}
	return snapshot, missing, requests
}
