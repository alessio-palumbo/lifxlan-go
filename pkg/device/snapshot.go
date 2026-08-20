package device

import (
	"github.com/alessio-palumbo/lifxlan-go/pkg/protocol"
	"github.com/alessio-palumbo/lifxprotocol-go/gen/protocol/packets"
)

// StateSnapshot is a target-free copy of restorable device state.
type StateSnapshot struct {
	Devices []DeviceStateSnapshot
}

// DeviceStateSnapshot is the restorable state for one device.
type DeviceStateSnapshot struct {
	Serial       Serial
	PoweredOn    bool
	Color        Color
	LightType    LightType
	Zones        []packets.LightHsbk
	MatrixChains [][]packets.LightHsbk
	MatrixWidth  int
}

// NewStateSnapshot returns a snapshot of restorable state from devices.
func NewStateSnapshot(devices []Device) StateSnapshot {
	snapshot := StateSnapshot{Devices: make([]DeviceStateSnapshot, 0, len(devices))}
	for _, d := range devices {
		snapshot.Devices = append(snapshot.Devices, NewDeviceStateSnapshot(d))
	}
	return snapshot
}

// NewDeviceStateSnapshot returns a snapshot of restorable state from d.
func NewDeviceStateSnapshot(d Device) DeviceStateSnapshot {
	return DeviceStateSnapshot{
		Serial:       d.Serial,
		PoweredOn:    d.PoweredOn,
		Color:        d.Color,
		LightType:    d.LightType,
		Zones:        CloneHSBKs(d.MultizoneProperties.Zones),
		MatrixChains: CloneMatrixChains(d.MatrixProperties.ChainZones),
		MatrixWidth:  d.MatrixProperties.Width,
	}
}

// RestorableStateMessages returns messages needed to refresh restorable state for
// d. Matrix devices require chain metadata before tile pixels can be requested.
func RestorableStateMessages(d Device) []*protocol.Message {
	if d.LightType == LightTypeMatrix && d.MatrixProperties.ChainLength == 0 {
		return []*protocol.Message{
			protocol.NewMessage(&packets.LightGet{}),
			protocol.NewMessage(&packets.DeviceGetPower{}),
			protocol.NewMessage(&packets.TileGetDeviceChain{}),
		}
	}
	return d.HighFreqStateMessages()
}

// RestorableStateReady reports whether d has enough cached state to restore.
func RestorableStateReady(d Device) bool {
	if d.LightType == LightTypeMatrix && d.PoweredOn {
		return MatrixChainStateReady(d)
	}
	return true
}

// MatrixChainStateReady reports whether d has cached colors for every known
// matrix chain.
func MatrixChainStateReady(d Device) bool {
	length := matrixSnapshotChainLength(d)
	if length < 1 || len(d.MatrixProperties.ChainZones) < length {
		return false
	}
	for _, colors := range d.MatrixProperties.ChainZones[:length] {
		if len(colors) == 0 {
			return false
		}
	}
	return true
}

// CloneHSBKs returns a copy of colors.
func CloneHSBKs(colors []packets.LightHsbk) []packets.LightHsbk {
	if len(colors) == 0 {
		return nil
	}
	return append([]packets.LightHsbk(nil), colors...)
}

// CloneMatrixChains returns a deep copy of chains.
func CloneMatrixChains(chains [][]packets.LightHsbk) [][]packets.LightHsbk {
	if len(chains) == 0 {
		return nil
	}
	cloned := make([][]packets.LightHsbk, len(chains))
	for i, colors := range chains {
		cloned[i] = CloneHSBKs(colors)
	}
	return cloned
}

func matrixSnapshotChainLength(d Device) int {
	if d.MatrixProperties.ChainLength > 0 {
		return d.MatrixProperties.ChainLength
	}
	if d.MatrixProperties.ChainZones != nil {
		return len(d.MatrixProperties.ChainZones)
	}
	return 0
}
