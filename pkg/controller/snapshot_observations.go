package controller

import (
	"github.com/alessio-palumbo/lifxlan-go/pkg/device"
	"github.com/alessio-palumbo/lifxprotocol-go/gen/protocol/packets"
)

// Receipt stamps distinguish unobserved buffers from valid black pixels. They
// also allow concurrent captures to choose their own freshness baseline without
// resetting shared coverage.
type snapshotObservations struct {
	generation uint64
	product    bool
	light      uint64
	zones      []uint64
	matrix     [][]uint64
	width      int
	height     int
	chains     int
}

// observe is called under the session mutex after applying the response.
func (o *snapshotObservations) observe(d *device.Device, payload packets.Payload) {
	switch p := payload.(type) {
	case *packets.DeviceStateVersion:
		o.product = p.Product != 0
	case *packets.LightState:
		o.generation++
		o.light = o.generation
	case *packets.TileStateDeviceChain:
		if p.TileDevicesCount == 0 {
			return
		}
		props := d.MatrixProperties
		if o.width != props.Width || o.height != props.Height || o.chains != props.ChainLength {
			o.width, o.height, o.chains = props.Width, props.Height, props.ChainLength
			o.matrix = make([][]uint64, props.ChainLength)
			for i := range o.matrix {
				o.matrix[i] = make([]uint64, props.StatePackets)
			}
		}
	case *packets.TileState64:
		props := d.MatrixProperties
		chain := int(p.TileIndex)
		if chain >= len(o.matrix) || props.Width <= 0 || p.Rect.FbIndex != 0 ||
			p.Rect.X != 0 || int(p.Rect.Width) != props.Width {
			return
		}
		offset := int(p.Rect.Y) * props.Width
		packet := offset / 64
		if offset%64 != 0 || packet >= len(o.matrix[chain]) ||
			chain >= len(props.ChainZones) || offset >= len(props.ChainZones[chain]) {
			return
		}
		o.generation++
		o.matrix[chain][packet] = o.generation
	case *packets.MultiZoneExtendedStateMultiZone:
		count, start := int(p.Count), int(p.Index)
		if count == 0 || int(p.ColorsCount) > len(p.Colors) {
			return
		}
		if len(o.zones) != count {
			o.zones = make([]uint64, count)
		}
		if start >= count || p.ColorsCount == 0 {
			return
		}
		o.generation++
		for i := start; i < min(count, start+int(p.ColorsCount)); i++ {
			o.zones[i] = o.generation
		}
	}
}

func observedCoverage(stamps []uint64, baseline uint64) bool {
	if len(stamps) == 0 {
		return false
	}
	for _, stamp := range stamps {
		if stamp <= baseline {
			return false
		}
	}
	return true
}

// missing reports the first prerequisite missing from this session. baseline=0
// permits complete cached state; other baselines require newer observations.
func (o *snapshotObservations) missing(d *device.Device, baseline uint64) string {
	if !o.product {
		return "product information"
	}
	if o.light <= baseline {
		return "light state"
	}
	switch d.LightType {
	case device.LightTypeMultiZone:
		if len(o.zones) != len(d.MultizoneProperties.Zones) || !observedCoverage(o.zones, baseline) {
			return "multizone coverage"
		}
	case device.LightTypeMatrix:
		props := d.MatrixProperties
		if o.width <= 0 || o.height <= 0 || o.chains <= 0 ||
			o.width != props.Width || o.height != props.Height || o.chains != props.ChainLength {
			return "matrix metadata"
		}
		if len(props.ChainZones) != o.chains || len(o.matrix) != o.chains {
			return "matrix coverage"
		}
		for i, stamps := range o.matrix {
			if len(props.ChainZones[i]) != props.Width*props.Height || !observedCoverage(stamps, baseline) {
				return "matrix coverage"
			}
		}
	}
	return ""
}
