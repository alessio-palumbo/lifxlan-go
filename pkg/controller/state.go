package controller

import (
	"errors"
	"fmt"
	"math"
	"time"

	"github.com/alessio-palumbo/lifxlan-go/pkg/device"
	"github.com/alessio-palumbo/lifxlan-go/pkg/messages"
	"github.com/alessio-palumbo/lifxlan-go/pkg/protocol"
	"github.com/alessio-palumbo/lifxprotocol-go/gen/protocol/enums"
)

var (
	// ErrDeviceNotFound indicates that no active session exists for a device.
	ErrDeviceNotFound = errors.New("device not found")
	// ErrUnsupportedCapability indicates that a state update targets a capability
	// the device does not expose.
	ErrUnsupportedCapability = errors.New("unsupported capability")
	// ErrInvalidState indicates that a state update contains an invalid value.
	ErrInvalidState = errors.New("invalid state")
)

// StateUpdate describes capability-scoped state changes for a device.
type StateUpdate struct {
	Light  *LightStateUpdate
	Relays []RelayStateUpdate
}

// LightStateUpdate describes light power and color changes. Nil color fields
// retain their current values.
type LightStateUpdate struct {
	Power      *bool
	Hue        *float64
	Saturation *float64
	Brightness *float64
	Kelvin     *uint16
	Duration   time.Duration
}

// RelayStateUpdate describes a switch relay power change.
type RelayStateUpdate struct {
	Index     int
	PoweredOn bool
}

// StateSendError reports a failure after zero or more state messages were sent.
// A non-zero Sent value means the device may have applied a partial update.
type StateSendError struct {
	Sent  int
	Total int
	Err   error
}

func (e *StateSendError) Error() string {
	return fmt.Sprintf("sent %d of %d state messages: %v", e.Sent, e.Total, e.Err)
}

// Unwrap returns the underlying transport error.
func (e *StateSendError) Unwrap() error {
	return e.Err
}

// SetState validates and sends a capability-scoped state update to a device.
// When color and power are provided together, color is sent before power.
func (c *Controller) SetState(serial device.Serial, update StateUpdate) error {
	c.mu.RLock()
	session, ok := c.sessions[serial]
	c.mu.RUnlock()
	if !ok {
		return fmt.Errorf("%w: %s", ErrDeviceNotFound, serial)
	}

	snapshot := session.deviceSnapshot()
	msgs, err := stateMessages(snapshot, update)
	if err != nil {
		return err
	}

	sent, err := session.sendWithProgress(msgs...)
	if err != nil {
		return &StateSendError{Sent: sent, Total: len(msgs), Err: err}
	}
	return nil
}

func stateMessages(d device.Device, update StateUpdate) ([]*protocol.Message, error) {
	if err := ValidateStateUpdate(update); err != nil {
		return nil, err
	}

	msgs := make([]*protocol.Message, 0, 2+len(update.Relays))
	if update.Light != nil {
		if d.Type == device.DeviceTypeSwitch {
			return nil, fmt.Errorf("%w: device %s has no light capability", ErrUnsupportedCapability, d.Serial)
		}
		lightMsgs, err := lightStateMessages(d, *update.Light)
		if err != nil {
			return nil, err
		}
		msgs = append(msgs, lightMsgs...)
	}

	if len(update.Relays) > 0 {
		if d.Type == device.DeviceTypeLight {
			return nil, fmt.Errorf("%w: device %s has no relay capability", ErrUnsupportedCapability, d.Serial)
		}
		for _, relay := range update.Relays {
			msgs = append(msgs, messages.SetRelayPower(relay.Index, relay.PoweredOn))
		}
	}

	return msgs, nil
}

// ValidateStateUpdate validates state values that do not depend on a target
// device's capabilities or color-temperature range.
func ValidateStateUpdate(update StateUpdate) error {
	if update.Light == nil && len(update.Relays) == 0 {
		return fmt.Errorf("%w: update contains no capabilities", ErrInvalidState)
	}
	if update.Light != nil {
		light := update.Light
		if light.Duration < 0 || light.Duration.Milliseconds() > math.MaxUint32 {
			return fmt.Errorf("%w: duration must be between 0 and %d milliseconds", ErrInvalidState, uint64(math.MaxUint32))
		}
		if err := validateRange("hue", light.Hue, 0, 360); err != nil {
			return err
		}
		if err := validateRange("saturation", light.Saturation, 0, 100); err != nil {
			return err
		}
		if err := validateRange("brightness", light.Brightness, 0, 100); err != nil {
			return err
		}
		hasColor := light.Hue != nil || light.Saturation != nil || light.Brightness != nil || light.Kelvin != nil
		if !hasColor && light.Power == nil {
			return fmt.Errorf("%w: light update contains no state fields", ErrInvalidState)
		}
	}

	seen := make(map[int]bool, len(update.Relays))
	for _, relay := range update.Relays {
		if relay.Index < 0 || relay.Index > math.MaxUint8 {
			return fmt.Errorf("%w: relay index %d must be between 0 and %d", ErrInvalidState, relay.Index, math.MaxUint8)
		}
		if seen[relay.Index] {
			return fmt.Errorf("%w: relay index %d is duplicated", ErrInvalidState, relay.Index)
		}
		seen[relay.Index] = true
	}
	return nil
}

func lightStateMessages(d device.Device, update LightStateUpdate) ([]*protocol.Message, error) {
	if update.Kelvin != nil {
		range_ := d.ColorProperties.TemperatureRange
		if range_.Min > 0 && range_.Max > 0 && (int(*update.Kelvin) < range_.Min || int(*update.Kelvin) > range_.Max) {
			return nil, fmt.Errorf("%w: kelvin must be between %d and %d", ErrInvalidState, range_.Min, range_.Max)
		}
	}

	hasColor := update.Hue != nil || update.Saturation != nil || update.Brightness != nil || update.Kelvin != nil
	msgs := make([]*protocol.Message, 0, 2)
	if hasColor {
		msgs = append(msgs, messages.SetColor(
			update.Hue,
			update.Saturation,
			update.Brightness,
			update.Kelvin,
			update.Duration,
			enums.LightWaveformLIGHTWAVEFORMSAW,
		))
	}
	if update.Power != nil {
		if *update.Power {
			if update.Duration > 0 {
				msgs = append(msgs, messages.SetPowerOn(update.Duration))
			} else {
				msgs = append(msgs, messages.SetPowerOn())
			}
		} else if update.Duration > 0 {
			msgs = append(msgs, messages.SetPowerOff(update.Duration))
		} else {
			msgs = append(msgs, messages.SetPowerOff())
		}
	}
	return msgs, nil
}

func validateRange(name string, value *float64, minValue, maxValue float64) error {
	if value == nil {
		return nil
	}
	if math.IsNaN(*value) || math.IsInf(*value, 0) || *value < minValue || *value > maxValue {
		return fmt.Errorf("%w: %s must be between %g and %g", ErrInvalidState, name, minValue, maxValue)
	}
	return nil
}
