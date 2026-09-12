// Package httpapi exposes the transport-independent control service over HTTP.
package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"net/http"
	"strings"
	"time"

	"github.com/alessio-palumbo/lifxlan-go/internal/control"
	"github.com/alessio-palumbo/lifxlan-go/pkg/controller"
	"github.com/alessio-palumbo/lifxlan-go/pkg/device"
)

const maxRequestBody = 1 << 20

type service interface {
	GetDevices() []device.Device
	GetDevice(device.Serial) (device.Device, bool)
	ApplyState([]string, controller.StateUpdate) ([]control.DeviceStateResult, error)
	SubscribeDevices(context.Context, ...controller.SubscriptionOption) <-chan controller.DeviceEvent
}

// NewHandler returns the HTTP API handler.
func NewHandler(service service) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", handleHealth)
	mux.HandleFunc("GET /v1/devices", func(w http.ResponseWriter, _ *http.Request) {
		handleGetDevices(w, service)
	})
	mux.HandleFunc("GET /v1/devices/events", func(w http.ResponseWriter, r *http.Request) {
		handleDeviceEvents(w, r, service)
	})
	mux.HandleFunc("PATCH /v1/devices/state", func(w http.ResponseWriter, r *http.Request) {
		handleSetState(w, r, service)
	})
	mux.HandleFunc("GET /v1/devices/{serial}", func(w http.ResponseWriter, r *http.Request) {
		handleGetDevice(w, r, service)
	})
	return mux
}

func handleHealth(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func handleGetDevices(w http.ResponseWriter, service service) {
	devices := service.GetDevices()
	responses := make([]deviceResponse, len(devices))
	for i, d := range devices {
		responses[i] = newDeviceResponse(d)
	}
	writeJSON(w, http.StatusOK, map[string]any{"devices": responses})
}

func handleGetDevice(w http.ResponseWriter, r *http.Request, service service) {
	serial, err := device.SerialFromHex(r.PathValue("serial"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_serial", err.Error())
		return
	}
	d, ok := service.GetDevice(serial)
	if !ok {
		writeError(w, http.StatusNotFound, "device_not_found", fmt.Sprintf("device %s was not found", serial))
		return
	}
	writeJSON(w, http.StatusOK, newDeviceResponse(d))
}

type stateRequest struct {
	Selectors []string            `json:"selectors"`
	Light     *lightStateRequest  `json:"light,omitempty"`
	Relays    []relayStateRequest `json:"relays,omitempty"`
}

type lightStateRequest struct {
	Power      *string            `json:"power,omitempty"`
	Color      *colorStateRequest `json:"color,omitempty"`
	DurationMS *int64             `json:"duration_ms,omitempty"`
}

type colorStateRequest struct {
	Hue        *float64 `json:"hue,omitempty"`
	Saturation *float64 `json:"saturation,omitempty"`
	Brightness *float64 `json:"brightness,omitempty"`
	Kelvin     *uint16  `json:"kelvin,omitempty"`
}

type relayStateRequest struct {
	Index int    `json:"index"`
	Power string `json:"power"`
}

type stateResultResponse struct {
	Serial string              `json:"serial"`
	Status control.ApplyStatus `json:"status"`
	Sent   int                 `json:"sent,omitempty"`
	Total  int                 `json:"total,omitempty"`
	Error  string              `json:"error,omitempty"`
}

func handleSetState(w http.ResponseWriter, r *http.Request, service service) {
	var request stateRequest
	if err := decodeJSON(w, r, &request); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_request", err.Error())
		return
	}
	update, err := request.stateUpdate()
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_state", err.Error())
		return
	}
	if err := controller.ValidateStateUpdate(update); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_state", err.Error())
		return
	}

	results, err := service.ApplyState(request.Selectors, update)
	if err != nil {
		switch {
		case errors.Is(err, control.ErrInvalidSelectors):
			writeError(w, http.StatusBadRequest, "invalid_selectors", err.Error())
		case errors.Is(err, control.ErrNoDevicesMatched):
			writeError(w, http.StatusNotFound, "no_devices_matched", err.Error())
		case errors.Is(err, control.ErrInvalidState):
			writeError(w, http.StatusBadRequest, "invalid_state", err.Error())
		default:
			writeError(w, http.StatusInternalServerError, "internal_error", "failed to apply device state")
		}
		return
	}

	response := make([]stateResultResponse, len(results))
	for i, result := range results {
		response[i] = stateResultResponse{
			Serial: result.Serial.String(),
			Status: result.Status,
			Sent:   result.Sent,
			Total:  result.Total,
		}
		if result.Err != nil {
			response[i].Error = result.Err.Error()
		}
	}
	writeJSON(w, http.StatusOK, map[string]any{"results": response})
}

func (r stateRequest) stateUpdate() (controller.StateUpdate, error) {
	update := controller.StateUpdate{}
	if r.Light != nil {
		light := controller.LightStateUpdate{}
		if r.Light.Power != nil {
			power, err := parsePower(*r.Light.Power)
			if err != nil {
				return update, fmt.Errorf("light power: %w", err)
			}
			light.Power = &power
		}
		if r.Light.Color != nil {
			light.Hue = r.Light.Color.Hue
			light.Saturation = r.Light.Color.Saturation
			light.Brightness = r.Light.Color.Brightness
			light.Kelvin = r.Light.Color.Kelvin
		}
		if r.Light.DurationMS != nil {
			if *r.Light.DurationMS < 0 || *r.Light.DurationMS > math.MaxUint32 {
				return update, fmt.Errorf("duration_ms must be between 0 and %d", uint64(math.MaxUint32))
			}
			light.Duration = time.Duration(*r.Light.DurationMS) * time.Millisecond
		}
		update.Light = &light
	}
	for _, relay := range r.Relays {
		power, err := parsePower(relay.Power)
		if err != nil {
			return update, fmt.Errorf("relay %d power: %w", relay.Index, err)
		}
		update.Relays = append(update.Relays, controller.RelayStateUpdate{
			Index:     relay.Index,
			PoweredOn: power,
		})
	}
	if update.Light == nil && len(update.Relays) == 0 {
		return update, errors.New("at least one light or relay state is required")
	}
	return update, nil
}

func parsePower(value string) (bool, error) {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "on":
		return true, nil
	case "off":
		return false, nil
	default:
		return false, fmt.Errorf("must be \"on\" or \"off\"")
	}
}

func decodeJSON(w http.ResponseWriter, r *http.Request, dst any) error {
	r.Body = http.MaxBytesReader(w, r.Body, maxRequestBody)
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(dst); err != nil {
		return err
	}
	if err := decoder.Decode(&struct{}{}); err != io.EOF {
		if err == nil {
			return errors.New("request body must contain one JSON value")
		}
		return err
	}
	return nil
}

func writeError(w http.ResponseWriter, status int, code, message string) {
	writeJSON(w, status, map[string]any{"error": map[string]string{"code": code, "message": message}})
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}
