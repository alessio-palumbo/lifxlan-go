package device

import "slices"

// Clone returns an independent copy of d. Mutating reference-backed fields on
// the returned Device does not alter the original.
func (d Device) Clone() Device {
	cloned := d

	if d.Address != nil {
		address := *d.Address
		address.IP = slices.Clone(d.Address.IP)
		cloned.Address = &address
	}

	cloned.MatrixProperties.ChainZones = CloneMatrixChains(d.MatrixProperties.ChainZones)
	cloned.MatrixProperties.ChainOrientations = slices.Clone(d.MatrixProperties.ChainOrientations)
	cloned.MatrixProperties.Effect.Palette = slices.Clone(d.MatrixProperties.Effect.Palette)
	cloned.MultizoneProperties.Zones = CloneHSBKs(d.MultizoneProperties.Zones)

	cloned.Buttons = slices.Clone(d.Buttons)
	for i := range cloned.Buttons {
		cloned.Buttons[i].Actions = slices.Clone(d.Buttons[i].Actions)
	}
	cloned.Relays = slices.Clone(d.Relays)

	return cloned
}
