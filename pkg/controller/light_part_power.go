package controller

import (
	"context"
	"errors"
	"fmt"
	"math"
	"reflect"
	"time"

	"github.com/alessio-palumbo/lifxlan-go/pkg/device"
	"github.com/alessio-palumbo/lifxlan-go/pkg/messages"
	"github.com/alessio-palumbo/lifxlan-go/pkg/protocol"
)

// ErrNoInheritedBrightness means no positive stored/active brightness is available.
var ErrNoInheritedBrightness = errors.New("no brightness available for part activation")

// LightPartPowerOptions configures explicit on/off intent, not ordinary colour writes.
type LightPartPowerOptions struct {
	Duration time.Duration
	// Timeout bounds the entire operation. Zero uses 3s plus Duration.
	Timeout time.Duration
	// FallbackBrightness is optional and must be finite, positive and <=100.
	// Nil returns ErrNoInheritedBrightness rather than inventing a brightness.
	FallbackBrightness *float64
}

// LightPartState distinguishes effective part state from device-wide power.
// Unknown observation coverage gives Known=false without fabricated brightness.
type LightPartState struct {
	Part            device.LightPart
	Known           bool
	DevicePoweredOn bool
	On              bool
	Brightness      float64
	Emulated        bool
}

// GetLightPartState reads observed cached state only. On uplight devices, part
// brightness is the arithmetic mean of visible selected emitters (black included)
// and all reports the brighter part. Other devices use reported global brightness.
func (c *Controller) GetLightPartState(serial device.Serial, part device.LightPart) (LightPartState, error) {
	part, err := normalizeLightPart(part)
	if err != nil {
		return LightPartState{}, err
	}
	c.mu.RLock()
	defer c.mu.RUnlock()
	s, ok := c.sessions[serial]
	if !ok {
		return LightPartState{}, ErrDeviceNotFound
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	d := s.device.Clone()
	if d.Type == device.DeviceTypeSwitch || part == device.LightPartUplight && !device.HasUplight(d) {
		return LightPartState{}, ErrUnsupportedCapability
	}
	state := LightPartState{Part: part, DevicePoweredOn: d.PoweredOn, Emulated: device.HasUplight(d)}
	if s.observations.missing(s.device, 0) != "" {
		return state, nil
	}
	if device.HasUplight(d) {
		main, up, err := partBrightness(d)
		if err != nil {
			return state, err
		}
		if part == device.LightPartMain {
			state.Brightness = main
		} else if part == device.LightPartUplight {
			state.Brightness = up
		} else {
			state.Brightness = max(main, up)
		}
		state.On = d.PoweredOn && state.Brightness > 0
	} else {
		state.Brightness = d.Color.Brightness
		state.On = d.PoweredOn
	}
	state.Known = true
	return state, nil
}

func normalizeLightPart(part device.LightPart) (device.LightPart, error) {
	if part == "" {
		part = device.LightPartAll
	}
	if part != device.LightPartAll && part != device.LightPartMain && part != device.LightPartUplight {
		return part, fmt.Errorf("%w: unknown light part %q", ErrInvalidState, part)
	}
	return part, nil
}

// SetLightPartPower expresses on/off intent. On current uplight ceilings this is
// emulated using brightness and shared device power; it is not native part power.
// It inherits active/retained brightness rather than restoring historical values.
// Fresh observations are required for emulation. Power-off is confirmed before
// preparing dormant uplight brightness; colours are prepared before power-on.
// No hidden backup buffers, control retries, rollback, or firmware effect stopping
// are used. External writers are not coordinated; failures can leave partial state.
// Devices without uplight use existing native device-power semantics.
func (c *Controller) SetLightPartPower(ctx context.Context, serial device.Serial, part device.LightPart, on bool, opts LightPartPowerOptions) error {
	if ctx == nil {
		ctx = context.Background()
	}
	part, err := normalizeLightPart(part)
	if err != nil {
		return err
	}
	if opts.Duration < 0 || opts.Duration > time.Duration(math.MaxUint32)*time.Millisecond || opts.Timeout < 0 {
		return fmt.Errorf("%w: invalid power timing", ErrInvalidState)
	}
	if opts.FallbackBrightness != nil && (math.IsNaN(*opts.FallbackBrightness) || math.IsInf(*opts.FallbackBrightness, 0) || *opts.FallbackBrightness <= 0 || *opts.FallbackBrightness > 100) {
		return fmt.Errorf("%w: fallback brightness must be >0 and <=100", ErrInvalidState)
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	timeout := opts.Timeout
	if timeout == 0 {
		timeout = defaultSnapshotTimeout + opts.Duration
	}
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	c.mu.RLock()
	s, ok := c.sessions[serial]
	c.mu.RUnlock()
	if !ok {
		return ErrDeviceNotFound
	}
	release, err := s.lockPartOperation(ctx)
	if err != nil {
		return err
	}
	defer release()
	s.mu.RLock()
	d := s.device.Clone()
	epoch := s.endpointEpoch
	s.mu.RUnlock()
	if d.Type == device.DeviceTypeSwitch || part == device.LightPartUplight && !device.HasUplight(d) {
		return ErrUnsupportedCapability
	}
	if !device.HasUplight(d) {
		msgs, err := lightStateMessages(d, LightStateUpdate{Power: &on, Duration: opts.Duration})
		if err != nil {
			return err
		}
		return c.sendPartPhase(ctx, serial, s, epoch, msgs)
	}
	if _, err := c.CaptureStateSnapshot(ctx, []device.Serial{serial}, SnapshotOptions{RequireFresh: true, Timeout: timeout}); err != nil {
		return fmt.Errorf("observe before part power: %w", err)
	}
	d, err = c.partOperationSnapshot(serial, s, epoch)
	if err != nil {
		return err
	}
	plan, err := planPartPower(d, part, on, opts)
	if err != nil {
		return err
	}
	if plan.powerOff {
		power := false
		msgs, _ := lightStateMessages(d, LightStateUpdate{Power: &power, Duration: opts.Duration})
		if err := c.sendPartPhase(ctx, serial, s, epoch, msgs); err != nil {
			return err
		}
		if plan.storeUplight != nil {
			plannedStore, err := lightPartMessages(d, device.LightPartUplight, LightStateUpdate{Brightness: plan.storeUplight})
			if err != nil {
				return &StateSendError{Sent: len(msgs), Total: len(msgs) + 1, Err: err}
			}
			total := len(msgs) + len(plannedStore)
			for {
				if _, err := c.CaptureStateSnapshot(ctx, []device.Serial{serial}, SnapshotOptions{RequireFresh: true, Timeout: timeout}); err != nil {
					return &StateSendError{Sent: len(msgs), Total: total, Err: fmt.Errorf("confirm off before dormant preparation: %w", err)}
				}
				d, err = c.partOperationSnapshot(serial, s, epoch)
				if err != nil {
					return &StateSendError{Sent: len(msgs), Total: total, Err: err}
				}
				if !d.PoweredOn {
					break
				}
				select {
				case <-ctx.Done():
					return &StateSendError{Sent: len(msgs), Total: total, Err: ctx.Err()}
				case <-time.After(25 * time.Millisecond):
				}
			}
			store, err := lightPartMessages(d, device.LightPartUplight, LightStateUpdate{Brightness: plan.storeUplight})
			if err != nil {
				return &StateSendError{Sent: len(msgs), Total: total, Err: err}
			}
			if err := c.sendPartPhase(ctx, serial, s, epoch, store); err != nil {
				var partial *StateSendError
				if errors.As(err, &partial) {
					return &StateSendError{Sent: len(msgs) + partial.Sent, Total: len(msgs) + partial.Total, Err: err}
				}
				return &StateSendError{Sent: len(msgs), Total: len(msgs) + len(store), Err: err}
			}
		}
		return nil
	}
	return c.sendPartPhase(ctx, serial, s, epoch, plan.messages)
}

func (c *Controller) partOperationSnapshot(serial device.Serial, s *deviceSession, epoch uint64) (device.Device, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	if c.sessions[serial] != s {
		return device.Device{}, ErrSessionClosed
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.endpointEpoch != epoch {
		return device.Device{}, ErrEndpointChanged
	}
	if reason := s.observations.missing(s.device, 0); reason != "" {
		return device.Device{}, fmt.Errorf("%w: observation invalidated (%s)", ErrInvalidState, reason)
	}
	return s.device.Clone(), nil
}

func (s *deviceSession) lockPartOperation(ctx context.Context) (func(), error) {
	s.partMu.Lock()
	if s.partGate == nil {
		s.partGate = make(chan struct{}, 1)
	}
	gate := s.partGate
	s.partMu.Unlock()
	select {
	case gate <- struct{}{}:
		if err := ctx.Err(); err != nil {
			<-gate
			return nil, err
		}
		return func() { <-gate }, nil
	case <-ctx.Done():
		return nil, ctx.Err()
	case <-s.done:
		return nil, ErrSessionClosed
	}
}

func (c *Controller) sendPartPhase(ctx context.Context, serial device.Serial, s *deviceSession, epoch uint64, msgs []*protocol.Message) error {
	if len(msgs) == 0 {
		return ctx.Err()
	}
	c.mu.RLock()
	defer c.mu.RUnlock()
	if c.sessions[serial] != s {
		return ErrSessionClosed
	}
	s.sendMu.Lock()
	defer s.sendMu.Unlock()
	s.mu.RLock()
	changed := s.endpointEpoch != epoch
	s.mu.RUnlock()
	if changed {
		return ErrEndpointChanged
	}
	sent, err := s.sendLocked(ctx, msgs...)
	if err != nil {
		return &StateSendError{Sent: sent, Total: len(msgs), Err: err}
	}
	return nil
}

type partPowerPlan struct {
	messages     []*protocol.Message
	powerOff     bool
	storeUplight *float64
}

func partBrightness(d device.Device) (float64, float64, error) {
	mean := func(part device.LightPart) (float64, error) {
		mask, err := lightPartMask(d, part)
		if err != nil {
			return 0, err
		}
		sum := 0.0
		count := 0
		for i, m := range mask {
			if m.Brightness > 0 {
				sum += float64(d.MatrixProperties.ChainZones[0][i].Brightness) / 65535 * 100
				count++
			}
		}
		if count == 0 {
			return 0, ErrUnsupportedCapability
		}
		return sum / float64(count), nil
	}
	main, err := mean(device.LightPartMain)
	if err != nil {
		return 0, 0, err
	}
	up, err := mean(device.LightPartUplight)
	return main, up, err
}

func planPartPower(d device.Device, part device.LightPart, on bool, opts LightPartPowerOptions) (partPowerPlan, error) {
	main, up, err := partBrightness(d)
	if err != nil {
		return partPowerPlan{}, err
	}
	if !on {
		if !d.PoweredOn {
			return partPowerPlan{}, nil
		}
		if part == device.LightPartAll || part == device.LightPartMain && up == 0 || part == device.LightPartUplight && main == 0 {
			plan := partPowerPlan{powerOff: true}
			if main > 0 && up == 0 {
				main = max(main, 100.0/65535)
				plan.storeUplight = &main
			}
			return plan, nil
		}
		if part == device.LightPartMain && main == 0 || part == device.LightPartUplight && up == 0 {
			return partPowerPlan{}, nil
		}
		zero := 0.0
		msgs, err := lightPartMessages(d, part, LightStateUpdate{Brightness: &zero, Duration: opts.Duration})
		return partPowerPlan{messages: msgs}, err
	}
	next := d.Clone()
	set := func(part device.LightPart, b float64) error {
		mask, err := lightPartMask(next, part)
		if err != nil {
			return err
		}
		for i, m := range mask {
			if m.Brightness > 0 {
				raw := device.ConvertExternalToDeviceValue(b, 100)
				if b > 0 && raw == 0 {
					raw = 1
				}
				next.MatrixProperties.ChainZones[0][i].Brightness = raw
			}
		}
		return nil
	}
	positive := func(own, other float64) (float64, error) {
		if own > 0 {
			return own, nil
		}
		if other > 0 {
			return other, nil
		}
		if opts.FallbackBrightness != nil {
			return *opts.FallbackBrightness, nil
		}
		return 0, ErrNoInheritedBrightness
	}
	if part == device.LightPartMain || part == device.LightPartAll {
		b, err := positive(main, up)
		if err != nil {
			return partPowerPlan{}, err
		}
		if main == 0 {
			if err := set(device.LightPartMain, b); err != nil {
				return partPowerPlan{}, err
			}
		}
	}
	if part == device.LightPartUplight || part == device.LightPartAll {
		b, err := positive(up, main)
		if err != nil {
			return partPowerPlan{}, err
		}
		if up == 0 {
			if err := set(device.LightPartUplight, b); err != nil {
				return partPowerPlan{}, err
			}
		}
	}
	if !d.PoweredOn && part != device.LightPartAll {
		other := device.LightPartMain
		if part == device.LightPartMain {
			other = device.LightPartUplight
		}
		if err := set(other, 0); err != nil {
			return partPowerPlan{}, err
		}
	}
	var msgs []*protocol.Message
	if !reflect.DeepEqual(next.MatrixProperties.ChainZones, d.MatrixProperties.ChainZones) {
		duration := opts.Duration
		if !d.PoweredOn {
			duration = 0
		}
		msgs = messages.SetMatrixColorsFromSlice(0, 1, d.MatrixProperties.Width, next.MatrixProperties.ChainZones[0], duration)
	}
	if !d.PoweredOn {
		power := true
		powerMsgs, _ := lightStateMessages(d, LightStateUpdate{Power: &power, Duration: opts.Duration})
		msgs = append(msgs, powerMsgs...)
	}
	return partPowerPlan{messages: msgs}, nil
}
