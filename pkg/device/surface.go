package device

import "slices"

// Surface describes a device's logical display surface and physical send layout.
type Surface struct {
	LightType LightType
	Width     int
	Height    int
	Zones     int
	Matrix    *MatrixSurface
}

// MatrixSurface describes a matrix device surface.
type MatrixSurface struct {
	Chains []MatrixChain
}

// MatrixChain describes one matrix in a matrix chain.
type MatrixChain struct {
	Index int
	// Bounds describes this chain's logical/display placement in cell units.
	Bounds Rect
	// SendWidth describes the matrix width used when sending packet colors.
	SendWidth int
	Rows      []MatrixRow
	// Orientation preserves the physical chain orientation reported by the device.
	Orientation Orientation
}

// Rect describes logical display placement in cell units.
type Rect struct {
	X      int
	Y      int
	Width  int
	Height int
}

// MatrixRow describes one logical display row in cell units.
type MatrixRow struct {
	Cols int
	// Offset is the logical cell offset for this row, not a pixel or UI offset.
	Offset     int
	HiddenCols []int
}

// SurfaceFromDevice derives a deterministic surface description from d.
func SurfaceFromDevice(d Device) Surface {
	switch d.LightType {
	case LightTypeMultiZone:
		zones := len(d.MultizoneProperties.Zones)
		return Surface{
			LightType: d.LightType,
			Width:     max(zones, 1),
			Height:    1,
			Zones:     max(zones, 1),
		}
	case LightTypeMatrix:
		return matrixSurfaceFromDevice(d)
	default:
		return Surface{
			LightType: d.LightType,
			Width:     1,
			Height:    1,
			Zones:     1,
		}
	}
}

func matrixSurfaceFromDevice(d Device) Surface {
	props := d.MatrixProperties
	chainCount := matrixChainCount(props)
	chains := make([]MatrixChain, 0, chainCount)

	var surfaceWidth, surfaceHeight, zones int
	for i := range chainCount {
		pixels := matrixChainPixels(props, i)
		displayWidth, displayHeight := displayMatrixDimensions(d.ProductID, props.Width, props.Height, pixels)
		rows := matrixRowsForProduct(d.ProductID, displayWidth, displayHeight)
		boundsWidth := matrixRowsWidth(rows, displayWidth)

		chain := MatrixChain{
			Index:       i,
			Bounds:      Rect{X: surfaceWidth, Y: 0, Width: boundsWidth, Height: len(rows)},
			SendWidth:   matrixSendWidth(props.Width, displayWidth),
			Rows:        cloneMatrixRows(rows),
			Orientation: matrixChainOrientation(props, i),
		}
		chains = append(chains, chain)

		surfaceWidth += boundsWidth
		surfaceHeight = max(surfaceHeight, chain.Bounds.Height)
		zones += max(pixels, displayWidth*displayHeight)
	}

	if surfaceWidth <= 0 {
		surfaceWidth = max(props.Width, 1)
	}
	if surfaceHeight <= 0 {
		surfaceHeight = max(props.Height, 1)
	}
	if zones <= 0 {
		zones = surfaceWidth * surfaceHeight
	}

	return Surface{
		LightType: d.LightType,
		Width:     surfaceWidth,
		Height:    surfaceHeight,
		Zones:     zones,
		Matrix:    &MatrixSurface{Chains: chains},
	}
}

func matrixChainCount(props MatrixProperties) int {
	if len(props.ChainZones) > 0 {
		return len(props.ChainZones)
	}
	if props.ChainLength > 0 {
		return props.ChainLength
	}
	return 1
}

func matrixChainPixels(props MatrixProperties, index int) int {
	if index >= 0 && index < len(props.ChainZones) && len(props.ChainZones[index]) > 0 {
		return len(props.ChainZones[index])
	}
	if props.NZones > 0 {
		return props.NZones
	}
	return max(props.Width, 1) * max(props.Height, 1)
}

func matrixChainOrientation(props MatrixProperties, index int) Orientation {
	if index >= 0 && index < len(props.ChainOrientations) {
		return props.ChainOrientations[index]
	}
	return OrientationRightSideUp
}

func matrixSendWidth(sendWidth, displayWidth int) int {
	if sendWidth > 0 {
		return sendWidth
	}
	return max(displayWidth, 1)
}

func displayMatrixDimensions(productID uint32, width, height, pixels int) (int, int) {
	width = max(width, 1)
	height = max(height, 1)
	if ceilingCapsuleProducts[productID] && pixels > 0 && pixels%16 == 0 {
		return 16, pixels / 16
	}
	return width, height
}

func matrixRowsForProduct(productID uint32, width, height int) []MatrixRow {
	rows := make([]MatrixRow, max(height, 0))
	for i := range rows {
		rows[i] = MatrixRow{Cols: width}
	}

	hiddenCols := hiddenMatrixColsByRow(productID, width)
	for rowIndex, cols := range hiddenCols {
		if rowIndex >= 0 && rowIndex < len(rows) {
			rows[rowIndex].HiddenCols = slices.Clone(cols)
		}
	}
	if candleProducts[productID] && len(rows) > 0 {
		rows[0].Offset = 1
	}

	return rows
}

func matrixRowsWidth(rows []MatrixRow, fallback int) int {
	width := fallback
	for _, row := range rows {
		width = max(width, row.Offset+row.Cols)
	}
	return max(width, 1)
}

func cloneMatrixRows(rows []MatrixRow) []MatrixRow {
	out := make([]MatrixRow, len(rows))
	for i, row := range rows {
		out[i] = row
		out[i].HiddenCols = slices.Clone(row.HiddenCols)
	}
	return out
}

func hiddenMatrixColsByRow(productID uint32, width int) map[int][]int {
	hidden := hiddenMatrixIndexes(productID)
	if len(hidden) == 0 || width <= 0 {
		return nil
	}
	byRow := make(map[int][]int)
	for _, index := range hidden {
		byRow[index/width] = append(byRow[index/width], index%width)
	}
	return byRow
}

func hiddenMatrixIndexes(productID uint32) []int {
	switch {
	case candleProducts[productID]:
		return []int{2, 3, 4}
	case ceilingProducts[productID]:
		return []int{0, 1, 6, 7, 56, 57, 62, 63}
	case ceilingCapsuleProducts[productID]:
		return []int{0, 1, 14, 15, 112, 113, 126, 127}
	case lunaProducts[productID]:
		return []int{0, 6, 28, 34}
	default:
		return nil
	}
}

var candleProducts = map[uint32]bool{
	57: true, 67: true, 68: true, 81: true, 96: true, 137: true, 138: true, 185: true, 186: true, 187: true, 188: true, 215: true, 216: true, 217: true, 218: true,
}

var ceilingProducts = map[uint32]bool{
	145: true, 146: true, 176: true, 177: true, 265: true, 266: true,
}

var ceilingCapsuleProducts = map[uint32]bool{
	201: true, 202: true,
}

var lunaProducts = map[uint32]bool{
	219: true, 220: true,
}
