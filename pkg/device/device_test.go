package device

import (
	"math"
	"testing"

	"github.com/alessio-palumbo/lifxprotocol-go/gen/protocol/packets"
	"github.com/stretchr/testify/assert"
)

func TestSetProductInfo(t *testing.T) {
	testCases := map[string]struct {
		pid  uint32
		want *Device
	}{
		"Single zone light": {
			pid: 97,
			want: &Device{
				ProductID:    97,
				RegistryName: "LIFX A19",
				LightType:    LightTypeSingleZone,
			},
		},
		"Multizone light": {
			pid: 117,
			want: &Device{
				ProductID:    117,
				RegistryName: "LIFX Z US",
				LightType:    LightTypeMultiZone,
			},
		},
		"Matrix light": {
			pid: 55,
			want: &Device{
				ProductID:    55,
				RegistryName: "LIFX Tile",
				LightType:    LightTypeMatrix,
			},
		},
		"Switch": {
			pid: 89,
			want: &Device{
				ProductID:    89,
				RegistryName: "LIFX Switch",
				Type:         DeviceTypeSwitch,
			},
		},
		"Hybrid": {
			pid: 219,
			want: &Device{
				ProductID:    219,
				RegistryName: "LIFX Luna US",
				Type:         DeviceTypeHybrid,
				LightType:    LightTypeMatrix,
			},
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			d := &Device{}
			d.SetProductInfo(tc.pid)
			assert.Equal(t, tc.want, d)
		})
	}
}

func TestSortDevices(t *testing.T) {
	var (
		serial0 = Serial([8]byte{0, 0, 0, 0, 0, 0, 0, 0})
		serial1 = Serial([8]byte{1, 0, 0, 0, 0, 0, 0, 0})
	)
	testCases := map[string]struct {
		devices []Device
		want    []Device
	}{
		"devices with different label": {
			devices: []Device{{Serial: serial0, Label: "B"}, {Serial: serial1, Label: "A"}},
			want:    []Device{{Serial: serial1, Label: "A"}, {Serial: serial0, Label: "B"}},
		},
		"devices with same label": {
			devices: []Device{{Serial: serial1, Label: "A"}, {Serial: serial0, Label: "A"}},
			want:    []Device{{Serial: serial0, Label: "A"}, {Serial: serial1, Label: "A"}},
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			SortDevices(tc.devices)
			assert.Equal(t, tc.want, tc.devices)
		})
	}
}

func TestSetMatrixProperties(t *testing.T) {
	tests := map[string]struct {
		device *Device
		msg    *packets.TileStateDeviceChain
		want   *Device
	}{
		"bad message": {
			device: &Device{},
			msg:    &packets.TileStateDeviceChain{},
			want:   &Device{},
		},
		"sets properties": {
			device: &Device{},
			msg: &packets.TileStateDeviceChain{
				TileDevices:      [16]packets.TileStateDevice{{Width: 8, Height: 8}, {Width: 8, Height: 8}},
				TileDevicesCount: 2,
			},
			want: &Device{
				MatrixProperties: MatrixProperties{
					Height: 8, Width: 8, ChainLength: 2,
					ChainState: [][64]packets.LightHsbk{{}, {}},
				},
			},
		},
		"sets properties when start at offset": {
			device: &Device{},
			msg: &packets.TileStateDeviceChain{
				StartIndex:       2,
				TileDevices:      [16]packets.TileStateDevice{{}, {}, {Width: 8, Height: 8}},
				TileDevicesCount: 1,
			},
			want: &Device{
				MatrixProperties: MatrixProperties{
					Height: 8, Width: 8, ChainLength: 1,
					ChainState: [][64]packets.LightHsbk{{}},
				},
			},
		},
		"adds tile to existing chain": {
			device: &Device{MatrixProperties: MatrixProperties{ChainState: [][64]packets.LightHsbk{{}}}},
			msg: &packets.TileStateDeviceChain{
				StartIndex:       0,
				TileDevices:      [16]packets.TileStateDevice{{Width: 8, Height: 8}, {Width: 8, Height: 8}},
				TileDevicesCount: 2,
			},
			want: &Device{
				MatrixProperties: MatrixProperties{
					Height: 8, Width: 8, ChainLength: 2,
					ChainState: [][64]packets.LightHsbk{{}, {}},
				},
			},
		},
		"deletes tile from existing chain": {
			device: &Device{MatrixProperties: MatrixProperties{ChainState: [][64]packets.LightHsbk{{}, {}}}},
			msg: &packets.TileStateDeviceChain{
				StartIndex:       0,
				TileDevices:      [16]packets.TileStateDevice{{Width: 8, Height: 8}},
				TileDevicesCount: 1,
			},
			want: &Device{
				MatrixProperties: MatrixProperties{
					Height: 8, Width: 8, ChainLength: 1,
					ChainState: [][64]packets.LightHsbk{{}},
				},
			},
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			tc.device.SetMatrixProperties(tc.msg)
			assert.Equal(t, tc.want, tc.device)
		})
	}
}

func TestSetMatrixState(t *testing.T) {

	color0 := packets.LightHsbk{Hue: 180, Saturation: math.MaxUint16, Brightness: math.MaxUint16, Kelvin: 3500}
	tile0 := [64]packets.LightHsbk{color0}

	tests := map[string]struct {
		device *Device
		msg    *packets.TileState64
		want   *Device
	}{
		"device has no matrix properties": {
			device: &Device{},
			msg: &packets.TileState64{
				TileIndex: 0, Colors: [64]packets.LightHsbk{},
			},
			want: &Device{},
		},
		"sets matrix state": {
			device: &Device{
				MatrixProperties: MatrixProperties{
					Height: 8, Width: 8, ChainLength: 2,
					ChainState: [][64]packets.LightHsbk{{}, {}},
				},
			},
			msg: &packets.TileState64{
				TileIndex: 0, Colors: [64]packets.LightHsbk{color0},
			},
			want: &Device{
				MatrixProperties: MatrixProperties{
					Height: 8, Width: 8, ChainLength: 2,
					ChainState: [][64]packets.LightHsbk{tile0, {}},
				},
			},
		},
		"sets matrix state at offset": {
			device: &Device{
				MatrixProperties: MatrixProperties{
					Height: 8, Width: 8, ChainLength: 2,
					ChainState: [][64]packets.LightHsbk{{}, {}},
				},
			},
			msg: &packets.TileState64{
				TileIndex: 1, Colors: [64]packets.LightHsbk{color0},
			},
			want: &Device{
				MatrixProperties: MatrixProperties{
					Height: 8, Width: 8, ChainLength: 2,
					ChainState: [][64]packets.LightHsbk{{}, tile0},
				},
			},
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			tc.device.SetMatrixState(tc.msg)
			assert.Equal(t, tc.want, tc.device)
		})
	}
}
