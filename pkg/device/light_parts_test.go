package device

import "testing"

func TestLightPartsMetadata(t *testing.T) {
	d := Device{ProductID: 265, Type: DeviceTypeLight, LightType: LightTypeMatrix, MatrixProperties: MatrixProperties{Width: 8, Height: 8, ChainLength: 1}}
	parts := LightParts(d)
	if len(parts) != 2 || parts[0].Part != LightPartMain || parts[0].ZoneCount != 56 || !parts[0].ZonesKnown || parts[1].Part != LightPartUplight || parts[1].ZoneCount != 1 {
		t.Fatal(parts)
	}
	d.MatrixProperties = MatrixProperties{}
	parts = LightParts(d)
	if parts[0].ZonesKnown || !parts[1].ZonesKnown {
		t.Fatal(parts)
	}
	d.ProductID = 215
	d.MatrixProperties = MatrixProperties{Width: 5, Height: 6, ChainLength: 1}
	parts = LightParts(d)
	if len(parts) != 1 || parts[0].ZoneCount != 27 || HasUplight(d) {
		t.Fatal(parts)
	}
	d.Type = DeviceTypeSwitch
	if len(LightParts(d)) != 0 || HasUplight(d) {
		t.Fatal("switch has parts")
	}
}
