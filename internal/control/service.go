// Package control provides a transport-independent application boundary around
// the LIFX LAN controller.
package control

import (
	"context"
	"errors"
	"fmt"

	"github.com/alessio-palumbo/lifxlan-go/pkg/controller"
	"github.com/alessio-palumbo/lifxlan-go/pkg/device"
)

var (
	// ErrInvalidSelectors indicates that the selector list is empty or malformed.
	ErrInvalidSelectors = errors.New("invalid selectors")
	// ErrNoDevicesMatched indicates that valid selectors matched no active devices.
	ErrNoDevicesMatched = errors.New("no devices matched")
	// ErrInvalidState indicates that a state update is structurally invalid.
	ErrInvalidState = errors.New("invalid state")
)

// Backend is the controller functionality used by Service.
type Backend interface {
	GetDevice(device.Serial) (device.Device, bool)
	GetDevices() []device.Device
	SetState(device.Serial, controller.StateUpdate) error
	SubscribeDevices(context.Context, ...controller.SubscriptionOption) <-chan controller.DeviceEvent
}

// Service applies transport-independent operations to a controller backend.
type Service struct {
	backend Backend
}

// New returns a control service backed by a controller.
func New(backend Backend) *Service {
	return &Service{backend: backend}
}

// GetDevices returns snapshots of all active devices.
func (s *Service) GetDevices() []device.Device {
	return s.backend.GetDevices()
}

// SubscribeDevices returns the backend's stream of observed device changes.
func (s *Service) SubscribeDevices(ctx context.Context, opts ...controller.SubscriptionOption) <-chan controller.DeviceEvent {
	return s.backend.SubscribeDevices(ctx, opts...)
}

// GetDevice returns the snapshot for an active device.
func (s *Service) GetDevice(serial device.Serial) (device.Device, bool) {
	return s.backend.GetDevice(serial)
}

// ApplyStatus describes the outcome of a state update for one device.
type ApplyStatus string

const (
	ApplyAccepted ApplyStatus = "accepted"
	ApplyPartial  ApplyStatus = "partial"
	ApplyRejected ApplyStatus = "rejected"
)

// DeviceStateResult describes the outcome for one selected device.
type DeviceStateResult struct {
	Serial device.Serial
	Status ApplyStatus
	Sent   int
	Total  int
	Err    error
}

// ApplyState resolves selectors against a single inventory snapshot and applies
// the update to every matched device. Unsupported capabilities are reported as
// per-device rejections rather than silently skipped.
func (s *Service) ApplyState(selectors []string, update controller.StateUpdate) ([]DeviceStateResult, error) {
	if len(selectors) == 0 {
		return nil, fmt.Errorf("%w: at least one selector is required", ErrInvalidSelectors)
	}

	selected, err := device.ResolveSelectorList(selectors, s.backend.GetDevices())
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrInvalidSelectors, err)
	}
	if len(selected) == 0 {
		return nil, ErrNoDevicesMatched
	}
	if err := controller.ValidateStateUpdate(update); err != nil {
		return nil, fmt.Errorf("%w: %v", ErrInvalidState, err)
	}

	results := make([]DeviceStateResult, 0, len(selected))
	for _, d := range selected {
		result := DeviceStateResult{Serial: d.Serial, Status: ApplyAccepted}
		err := s.backend.SetState(d.Serial, update)
		if err != nil {
			result.Status = ApplyRejected
			result.Err = err

			var sendErr *controller.StateSendError
			if errors.As(err, &sendErr) {
				result.Sent = sendErr.Sent
				result.Total = sendErr.Total
				if sendErr.Sent > 0 {
					result.Status = ApplyPartial
				}
			}
		}
		results = append(results, result)
	}
	return results, nil
}
