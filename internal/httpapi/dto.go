package httpapi

import (
	"time"

	"github.com/alessio-palumbo/lifxlan-go/pkg/controller"
	"github.com/alessio-palumbo/lifxlan-go/pkg/device"
)

type deviceResponse struct {
	Serial            string          `json:"serial"`
	Label             string          `json:"label"`
	Type              string          `json:"type"`
	ProductID         uint32          `json:"product_id"`
	ProductName       string          `json:"product_name,omitempty"`
	RegistryKnown     bool            `json:"registry_known"`
	FirmwareVersion   string          `json:"firmware_version,omitempty"`
	LocationID        string          `json:"location_id,omitempty"`
	Location          string          `json:"location"`
	GroupID           string          `json:"group_id,omitempty"`
	Group             string          `json:"group"`
	EstimatedBootedAt *time.Time      `json:"estimated_booted_at,omitempty"`
	Light             *lightResponse  `json:"light,omitempty"`
	Switch            *switchResponse `json:"switch,omitempty"`
	LastSeenAt        *time.Time      `json:"last_seen_at,omitempty"`
	LastUpdatedAt     *time.Time      `json:"last_updated_at,omitempty"`
}

type lightResponse struct {
	Type      string          `json:"type"`
	Power     string          `json:"power"`
	Color     colorResponse   `json:"color"`
	HasColor  bool            `json:"has_color"`
	MinKelvin int             `json:"min_kelvin,omitempty"`
	MaxKelvin int             `json:"max_kelvin,omitempty"`
	Effect    *effectResponse `json:"effect,omitempty"`
}

type effectResponse struct {
	Type                string          `json:"type"`
	Running             bool            `json:"running"`
	InstanceID          uint32          `json:"instance_id"`
	SpeedMS             int64           `json:"speed_ms"`
	RemainingDurationMS int64           `json:"remaining_duration_ms"`
	Direction           string          `json:"direction,omitempty"`
	Variant             string          `json:"variant,omitempty"`
	Palette             []colorResponse `json:"palette,omitempty"`
}

type colorResponse struct {
	Hue        float64 `json:"hue"`
	Saturation float64 `json:"saturation"`
	Brightness float64 `json:"brightness"`
	Kelvin     uint16  `json:"kelvin"`
}

type switchResponse struct {
	Relays []relayResponse `json:"relays"`
}

type relayResponse struct {
	Index int    `json:"index"`
	Power string `json:"power"`
}

type deviceEventResponse struct {
	Type     string          `json:"type"`
	Revision uint64          `json:"revision"`
	Initial  bool            `json:"initial,omitempty"`
	Changes  []string        `json:"changes,omitempty"`
	Device   *deviceResponse `json:"device,omitempty"`
}

func newDeviceResponse(d device.Device) deviceResponse {
	response := deviceResponse{
		Serial:          d.Serial.String(),
		Label:           d.Label,
		Type:            d.Type.String(),
		ProductID:       d.ProductID,
		ProductName:     d.RegistryName,
		RegistryKnown:   d.RegistryKnown,
		FirmwareVersion: d.FirmwareVersion,
		Location:        d.Location,
		Group:           d.Group,
	}
	if !d.LocationID.IsNil() {
		response.LocationID = d.LocationID.String()
	}
	if !d.GroupID.IsNil() {
		response.GroupID = d.GroupID.String()
	}
	if !d.EstimatedBootedAt.IsZero() {
		bootedAt := d.EstimatedBootedAt
		response.EstimatedBootedAt = &bootedAt
	}
	if !d.LastSeenAt.IsZero() {
		seen := d.LastSeenAt
		response.LastSeenAt = &seen
	}
	if !d.LastUpdatedAt.IsZero() {
		updated := d.LastUpdatedAt
		response.LastUpdatedAt = &updated
	}

	if d.Type != device.DeviceTypeSwitch {
		light := &lightResponse{
			Type:  d.LightType.String(),
			Power: powerString(d.PoweredOn),
			Color: colorResponse{
				Hue:        d.Color.Hue,
				Saturation: d.Color.Saturation,
				Brightness: d.Color.Brightness,
				Kelvin:     d.Color.Kelvin,
			},
			HasColor:  d.ColorProperties.HasColor,
			MinKelvin: d.ColorProperties.TemperatureRange.Min,
			MaxKelvin: d.ColorProperties.TemperatureRange.Max,
		}
		switch d.LightType {
		case device.LightTypeMultiZone:
			if effect := d.MultizoneProperties.Effect; effect.Known {
				light.Effect = &effectResponse{
					Type: effect.Type.String(), Running: effect.Running(), InstanceID: effect.InstanceID,
					SpeedMS: effect.Speed.Milliseconds(), RemainingDurationMS: effect.RemainingDuration.Milliseconds(),
				}
				if effect.Type == device.MultizoneEffectTypeMove {
					light.Effect.Direction = effect.Direction.String()
				}
			}
		case device.LightTypeMatrix:
			if effect := d.MatrixProperties.Effect; effect.Known {
				light.Effect = &effectResponse{
					Type: effect.Type.String(), Running: effect.Running(), InstanceID: effect.InstanceID,
					SpeedMS: effect.Speed.Milliseconds(), RemainingDurationMS: effect.RemainingDuration.Milliseconds(),
					Palette: make([]colorResponse, len(effect.Palette)),
				}
				if effect.Type == device.MatrixEffectTypeSky {
					light.Effect.Variant = effect.SkyType.String()
				}
				for i, color := range effect.Palette {
					light.Effect.Palette[i] = colorResponse{
						Hue: color.Hue, Saturation: color.Saturation, Brightness: color.Brightness, Kelvin: color.Kelvin,
					}
				}
			}
		}
		response.Light = light
	}
	if d.Type != device.DeviceTypeLight {
		relays := make([]relayResponse, len(d.Relays))
		for i, relay := range d.Relays {
			relays[i] = relayResponse{Index: relay.Index, Power: powerString(relay.PoweredOn)}
		}
		response.Switch = &switchResponse{Relays: relays}
	}
	return response
}

func powerString(poweredOn bool) string {
	if poweredOn {
		return "on"
	}
	return "off"
}

func newDeviceEventResponse(event controller.DeviceEvent) deviceEventResponse {
	response := deviceEventResponse{
		Type:     event.Type.String(),
		Revision: event.Revision,
		Initial:  event.Initial,
		Changes:  deviceChangeNames(event.Changes),
	}
	if event.Type != controller.DeviceEventResyncRequired && event.Type != controller.DeviceEventSnapshotComplete {
		device := newDeviceResponse(event.Device)
		response.Device = &device
	}
	return response
}

func deviceChangeNames(changes controller.DeviceChange) []string {
	known := []struct {
		change controller.DeviceChange
		name   string
	}{
		{controller.DeviceChangeLabel, "label"},
		{controller.DeviceChangeProduct, "product"},
		{controller.DeviceChangeFirmware, "firmware"},
		{controller.DeviceChangeLocation, "location"},
		{controller.DeviceChangeGroup, "group"},
		{controller.DeviceChangeWiFi, "wifi"},
		{controller.DeviceChangeLight, "light"},
		{controller.DeviceChangeMatrix, "matrix"},
		{controller.DeviceChangeMultizone, "multizone"},
		{controller.DeviceChangeButtons, "buttons"},
		{controller.DeviceChangeButtonConfig, "button_config"},
		{controller.DeviceChangeRelays, "relays"},
		{controller.DeviceChangeEffect, "effect"},
		{controller.DeviceChangeUptime, "uptime"},
	}
	var names []string
	for _, item := range known {
		if changes.Has(item.change) {
			names = append(names, item.name)
		}
	}
	return names
}
