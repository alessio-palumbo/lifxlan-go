package device

import "math"

// VisibleZoneCount returns the number of visible light cells in cached geometry.
// The boolean is false for non-light devices or missing/invalid geometry.
// Matrix hidden cells and logical padding are excluded using the same product
// mapping as SurfaceFromDevice. Physical buffers and Surface.Zones are unchanged.
// This is not a check that color state has been observed or is fresh.
func VisibleZoneCount(d Device) (int, bool) {
	if d.Type != DeviceTypeLight && d.Type != DeviceTypeHybrid {
		return 0, false
	}
	switch d.LightType {
	case LightTypeSingleZone:
		if d.ProductID != 0 {
			return 1, true
		}
	case LightTypeMultiZone:
		count := len(d.MultizoneProperties.Zones)
		return count, count > 0
	case LightTypeMatrix:
		p := d.MatrixProperties
		if p.Width <= 0 || p.Height <= 0 || p.ChainLength <= 0 || p.Width > math.MaxInt/p.Height {
			return 0, false
		}
		// Metadata-only chains have identical geometry. Count them in O(1),
		// without allocating rows or looping over an untrusted chain count.
		if len(p.ChainZones) == 0 {
			count, ok := visibleMatrixChainZones(d, 0)
			if !ok || count > math.MaxInt/p.ChainLength {
				return 0, false
			}
			return count * p.ChainLength, true
		}
		count := 0
		for chain := range matrixChainCount(p) {
			cells, ok := visibleMatrixChainZones(d, chain)
			if !ok || cells > math.MaxInt-count {
				return 0, false
			}
			count += cells
		}
		return count, count > 0
	}
	return 0, false
}

func visibleMatrixChainZones(d Device, index int) (int, bool) {
	p := d.MatrixProperties
	width, height := displayMatrixDimensions(d.ProductID, p.Width, p.Height, matrixChainPixels(p, index))
	if width <= 0 || height <= 0 || width > math.MaxInt/height {
		return 0, false
	}
	physicalCells := width * height
	visible := physicalCells
	// Row offsets affect placement, not the number of emitters. Hidden indexes
	// use the same display width as matrixRowsForProduct/SurfaceFromDevice.
	for _, hidden := range hiddenMatrixIndexes(d.ProductID) {
		if hidden >= 0 && hidden < physicalCells {
			visible--
		}
	}
	return visible, visible > 0
}
