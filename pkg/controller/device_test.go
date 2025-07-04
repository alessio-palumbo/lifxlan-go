package controller

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestSetProductInfo(t *testing.T) {
	tests := map[string]struct {
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
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			d := &Device{}
			d.SetProductInfo(tt.pid)
			assert.Equal(t, tt.want, d)
		})
	}
}

func TestSortDevices(t *testing.T) {
	var (
		serial0 = Serial([8]byte{0, 0, 0, 0, 0, 0, 0, 0})
		serial1 = Serial([8]byte{1, 0, 0, 0, 0, 0, 0, 0})
	)
	tests := map[string]struct {
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

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			SortDevices(tt.devices)
			assert.Equal(t, tt.want, tt.devices)
		})
	}
}
