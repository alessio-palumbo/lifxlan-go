package device

import (
	"net"
	"testing"

	"github.com/alessio-palumbo/lifxprotocol-go/gen/protocol/packets"
)

func TestDeviceCloneIsIndependent(t *testing.T) {
	original := Device{
		Address: &net.UDPAddr{IP: net.IP{192, 168, 1, 10}, Port: 56700},
		MatrixProperties: MatrixProperties{
			ChainZones:        [][]packets.LightHsbk{{{Hue: 1}}},
			ChainOrientations: []Orientation{OrientationLeft},
		},
		MultizoneProperties: MultizoneProperties{
			Zones: []packets.LightHsbk{{Hue: 2}},
		},
		Buttons: []Button{{Actions: []packets.ButtonAction{{}}}},
		Relays:  []Relay{{Index: 1, PoweredOn: true}},
	}

	cloned := original.Clone()
	cloned.Address.IP[0] = 10
	cloned.MatrixProperties.ChainZones[0][0].Hue = 10
	cloned.MatrixProperties.ChainOrientations[0] = OrientationRight
	cloned.MultizoneProperties.Zones[0].Hue = 20
	cloned.Buttons[0].Actions[0].TargetType = 1
	cloned.Relays[0].PoweredOn = false

	if original.Address.IP[0] != 192 {
		t.Fatal("clone shares address IP storage")
	}
	if original.MatrixProperties.ChainZones[0][0].Hue != 1 {
		t.Fatal("clone shares matrix zone storage")
	}
	if original.MatrixProperties.ChainOrientations[0] != OrientationLeft {
		t.Fatal("clone shares matrix orientation storage")
	}
	if original.MultizoneProperties.Zones[0].Hue != 2 {
		t.Fatal("clone shares multizone storage")
	}
	if original.Buttons[0].Actions[0].TargetType != 0 {
		t.Fatal("clone shares button action storage")
	}
	if !original.Relays[0].PoweredOn {
		t.Fatal("clone shares relay storage")
	}
}

func TestNilDeviceClone(t *testing.T) {
	cloned := (Device{}).Clone()
	if cloned.Address != nil || cloned.MatrixProperties.ChainZones != nil || cloned.Buttons != nil {
		t.Fatalf("zero-value clone = %#v", cloned)
	}
}
