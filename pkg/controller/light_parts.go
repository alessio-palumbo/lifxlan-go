package controller

import (
	"context"
	"fmt"

	"github.com/alessio-palumbo/lifxlan-go/pkg/device"
	"github.com/alessio-palumbo/lifxlan-go/pkg/effects"
	"github.com/alessio-palumbo/lifxlan-go/pkg/messages"
	"github.com/alessio-palumbo/lifxlan-go/pkg/protocol"
	"github.com/alessio-palumbo/lifxprotocol-go/gen/protocol/packets"
)

// SetLightPartColor applies partial HSBK values to all, main, or uplight. Empty part
// means all. Power must be nil; use SetLightPartPower for part on/off intent or
// SetState for native device-wide power.
// Part targeting uses complete observed physical state to preserve unselected
// cells. Snapshot options control timeout/freshness (cached complete state is
// accepted by default). It does not stop effects, reserve backup buffers, retry
// controls, or claim acknowledgement. Existing SetState semantics are unchanged.
func (c *Controller) SetLightPartColor(ctx context.Context, serial device.Serial, part device.LightPart, update LightStateUpdate, opts SnapshotOptions) error {
	if ctx == nil {
		ctx = context.Background()
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if part == "" {
		part = device.LightPartAll
	}
	if part != device.LightPartAll && part != device.LightPartMain && part != device.LightPartUplight {
		return fmt.Errorf("%w: unknown light part %q", ErrInvalidState, part)
	}
	if update.Power != nil {
		return fmt.Errorf("%w: SetLightPartColor does not accept power; use SetLightPartPower or SetState", ErrInvalidState)
	}
	if err := ValidateStateUpdate(StateUpdate{Light: &update}); err != nil {
		return err
	}
	c.mu.RLock()
	session, ok := c.sessions[serial]
	c.mu.RUnlock()
	if !ok {
		return fmt.Errorf("%w: %s", ErrDeviceNotFound, serial)
	}
	release, err := session.lockPartOperation(ctx)
	if err != nil {
		return err
	}
	defer release()
	session.mu.RLock()
	d := session.device.Clone()
	epoch := session.endpointEpoch
	session.mu.RUnlock()
	if d.Type == device.DeviceTypeSwitch || part == device.LightPartUplight && !device.HasUplight(d) {
		return fmt.Errorf("%w: light part %s", ErrUnsupportedCapability, part)
	}
	merge := part != device.LightPartAll && device.HasUplight(d)
	if merge {
		if _, err := c.CaptureStateSnapshot(ctx, []device.Serial{serial}, opts); err != nil {
			return fmt.Errorf("observe before part update: %w", err)
		}
	}
	c.mu.RLock()
	defer c.mu.RUnlock()
	if c.sessions[serial] != session {
		return ErrSessionClosed
	}
	session.sendMu.Lock()
	defer session.sendMu.Unlock()
	session.mu.RLock()
	d = session.device.Clone()
	changed := session.endpointEpoch != epoch
	missing := ""
	if merge {
		missing = session.observations.missing(session.device, 0)
	}
	session.mu.RUnlock()
	if changed {
		return ErrEndpointChanged
	}
	if missing != "" {
		return fmt.Errorf("%w: observation invalidated (%s)", ErrInvalidState, missing)
	}
	msgs, err := lightPartMessages(d, part, update)
	if err != nil {
		return err
	}
	sent, err := session.sendLocked(ctx, msgs...)
	if err != nil {
		return &StateSendError{Sent: sent, Total: len(msgs), Err: err}
	}
	return nil
}

func lightPartMessages(d device.Device, part device.LightPart, update LightStateUpdate) ([]*protocol.Message, error) {
	if _, err := lightStateMessages(d, update); err != nil {
		return nil, err
	} // shared capability/value validation
	if !d.ColorProperties.HasColor {
		zero := 0.0
		update.Saturation = &zero
	}
	if part == device.LightPartAll || part == device.LightPartMain && !device.HasUplight(d) {
		return lightStateMessages(d, update)
	}
	physicalMask, err := lightPartMask(d, part)
	if err != nil {
		return nil, err
	}
	colors := device.CloneHSBKs(d.MatrixProperties.ChainZones[0])
	for i, selected := range physicalMask {
		if selected.Brightness == 0 {
			continue
		}
		if update.Hue != nil {
			colors[i].Hue = device.ConvertExternalToDeviceValue(*update.Hue, 360)
		}
		if update.Saturation != nil {
			colors[i].Saturation = device.ConvertExternalToDeviceValue(*update.Saturation, 100)
		}
		if update.Brightness != nil {
			colors[i].Brightness = device.ConvertExternalToDeviceValue(*update.Brightness, 100)
		}
		if update.Kelvin != nil {
			colors[i].Kelvin = *update.Kelvin
		}
	}
	return messages.SetMatrixColorsFromSlice(0, 1, d.MatrixProperties.Width, colors, update.Duration), nil
}

func lightPartMask(d device.Device, part device.LightPart) ([]packets.LightHsbk, error) {
	if !device.HasUplight(d) || d.LightType != device.LightTypeMatrix {
		return nil, fmt.Errorf("%w: uplight mapping unavailable", ErrUnsupportedCapability)
	}
	mask, err := effects.LightPartPhysicalMask(d, part)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrInvalidState, err)
	}
	return mask, nil
}
