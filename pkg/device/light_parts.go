package device

import "github.com/alessio-palumbo/lifxregistry-go/gen/registry"

// LightPart selects logical light emitters, independently of protocol addressing.
type LightPart string

const (
	LightPartAll     LightPart = "all"
	LightPartMain    LightPart = "main"
	LightPartUplight LightPart = "uplight"
)

// LightPartInfo describes a real light part. All is a selector, not a part.
// Counts describe cached geometry, never observation completeness.
type LightPartInfo struct {
	Part             LightPart
	LightType        LightType
	ZoneCount        int
	ZonesKnown       bool
	HasColor         bool
	TemperatureRange TemperatureRange
}

// HasUplight reports registry capability, even before geometry/colors arrive.
func HasUplight(d Device) bool {
	if d.Type != DeviceTypeLight && d.Type != DeviceTypeHybrid {
		return false
	}
	p, known := registry.ProductsByPID[int(d.ProductID)]
	return known && p.Features.UplightCoords != nil
}

// LightParts returns independent metadata for the device's main and optional
// uplight. It performs no I/O, duplicates no serials, and exposes no addresses.
// The main count excludes hidden slots and uplight; uplight is currently one zone.
func LightParts(d Device) []LightPartInfo {
	if d.Type != DeviceTypeLight && d.Type != DeviceTypeHybrid {
		return nil
	}
	count, known := VisibleZoneCount(d)
	parts := []LightPartInfo{{Part: LightPartMain, LightType: d.LightType, ZoneCount: count, ZonesKnown: known, HasColor: d.ColorProperties.HasColor, TemperatureRange: d.ColorProperties.TemperatureRange}}
	if HasUplight(d) {
		parts = append(parts, LightPartInfo{Part: LightPartUplight, LightType: LightTypeSingleZone, ZoneCount: 1, ZonesKnown: true, HasColor: d.ColorProperties.HasColor, TemperatureRange: d.ColorProperties.TemperatureRange})
	}
	return parts
}
