package matrix

import (
	"time"

	"github.com/alessio-palumbo/lifxlan-go/pkg/messages"
	"github.com/alessio-palumbo/lifxlan-go/pkg/protocol"
	"github.com/alessio-palumbo/lifxprotocol-go/gen/protocol/packets"
)

var (
	minInterval = time.Millisecond
)

type chainMode int

const (
	// ChainModeNone applies the effect to the first device in the chain.
	ChainModeNone chainMode = iota
	// ChainModeSequential applies the effect sequentially on each chain index.
	ChainModeSequential
	// ChainModeSynced applies the effect to the whole chain.
	ChainModeSynced
)

// SendFunc is an interface for sending protocol messages.
type SendFunc = func(msg *protocol.Message) error

// Waterfall applies the given colors sequentially on each row centering them, if possible.
// It waits for the given interval before setting the next row.
// It repeats for n cycles, if cycles is set to 0 it repeats indefinitely.
func Waterfall(m *Matrix, send SendFunc, sendIntervalMs int64, cycles int, mode chainMode, colors ...packets.LightHsbk) {
	d := max(time.Duration(sendIntervalMs)*time.Millisecond, minInterval)

	if len(colors) > m.Width {
		colors = colors[:m.Width]
	}
	// Try to center the colors if possible.
	x := (m.Width - len(colors)) / 2

	repeatForCycles(cycles, func() {
		switch mode {
		case ChainModeSequential:
			for ti := range m.ChainLength {
				waterfall(m, send, d, x, ti, 1, colors...)
			}
		case ChainModeSynced:
			waterfall(m, send, d, x, 0, m.ChainLength, colors...)
		default:
			waterfall(m, send, d, x, 0, 1, colors...)
		}
	})
}

func waterfall(m *Matrix, send SendFunc, d time.Duration, x, mIdx, mLength int, colors ...packets.LightHsbk) {
	m.Clear()

	for i := range m.Height {
		m.SetColors(x, i, colors...)
		send(messages.SetMatrixColors(uint8(mIdx), uint8(mLength), uint8(m.Width), m.FlattenColors(), minInterval))
		time.Sleep(d)
	}
}

// Rockets sequentially applies a color to a single pixel row by row wrapping at the end.
// It waits for the given interval before setting the next pixel.
// If multiple colors are provided colors are rotated on each row.
// It repeats for n cycles, if cycles is set to 0 it repeats indefinitely.
func Rockets(m *Matrix, send SendFunc, sendIntervalMs int64, cycles int, mode chainMode, colors ...packets.LightHsbk) {
	d := max(time.Duration(sendIntervalMs)*time.Millisecond, minInterval)

	repeatForCycles(cycles, func() {
		switch mode {
		case ChainModeSequential:
			for ti := range m.ChainLength {
				rockets(m, send, d, ti, 1, colors...)
			}
		case ChainModeSynced:
			rockets(m, send, d, 0, m.ChainLength, colors...)
		default:
			rockets(m, send, d, 0, 1, colors...)
		}
	})
}

func rockets(m *Matrix, send SendFunc, d time.Duration, mIdx, mLength int, colors ...packets.LightHsbk) {
	m.Clear()

	color := colors[0]
	var y int
	for i := range m.Size {
		x := i % m.Width
		if i > 0 && x == 0 {
			y++
			color = colors[y%len(colors)]
		}
		m.Clear()

		m.SetPixel(x, y, color)
		send(messages.SetMatrixColors(uint8(mIdx), uint8(mLength), uint8(m.Width), m.FlattenColors(), minInterval))
		time.Sleep(d)
	}
}

// Worm moves n pixels along the matrix wrapping and reversing at every row with a wave-like movement.
// It repeats for n cycles, if cycles is set to 0 it repeats indefinitely.
func Worm(m *Matrix, send SendFunc, sendIntervalMs int64, cycles int, mode chainMode, size int, color packets.LightHsbk) {
	d := max(time.Duration(sendIntervalMs)*time.Millisecond, minInterval)
	wormSize := min(max(size, 1), m.Width)

	repeatForCycles(cycles, func() {
		switch mode {
		case ChainModeSequential:
			for ti := range m.ChainLength {
				worm(m, send, d, wormSize, ti, 1, color)
			}
		case ChainModeSynced:
			worm(m, send, d, wormSize, 0, m.ChainLength, color)
		default:
			worm(m, send, d, wormSize, 0, 1, color)
		}
	})
}

func worm(m *Matrix, send SendFunc, d time.Duration, wormSize, mIdx, mLength int, color packets.LightHsbk) {
	m.Clear()

	pxCache := NewPixelCache(wormSize)
	var pixelsSet int

	var x, y int
	var reversed bool
	for i := range m.Size {
		x, y = nextPixel(m, i, y, &reversed)
		if pixelsSet == wormSize {
			m.Clear(pxCache.Pixels()...)
			pixelsSet = 0
		}
		pixelsSet++
		pxCache.SetPixel(i%wormSize, x, y)

		m.SetPixel(x, y, color)
		send(messages.SetMatrixColors(uint8(mIdx), uint8(mLength), uint8(m.Width), m.FlattenColors(), minInterval))
		time.Sleep(d)
	}

	// Clear the tail and turn off all pixels.
	for _, p := range pxCache.Pixels() {
		m.Clear(p)
		send(messages.SetMatrixColors(uint8(mIdx), uint8(mLength), uint8(m.Width), m.FlattenColors(), minInterval))
		time.Sleep(d)
	}
}

// Snake moves n pixels along the matrix wrapping and reversing at every row.
// It repeats for n cycles, if cycles is set to 0 it repeats indefinitely.
func Snake(m *Matrix, send SendFunc, sendIntervalMs int64, cycles int, mode chainMode, size int, color packets.LightHsbk) {
	d := max(time.Duration(sendIntervalMs)*time.Millisecond, minInterval)
	snakeSize := min(max(size, 1), m.Width)

	repeatForCycles(cycles, func() {
		switch mode {
		case ChainModeSequential:
			for ti := range m.ChainLength {
				snake(m, send, d, snakeSize, ti, 1, color)
			}
		case ChainModeSynced:
			snake(m, send, d, snakeSize, 0, m.ChainLength, color)
		default:
			snake(m, send, d, snakeSize, 0, 1, color)
		}
	})
}

func snake(m *Matrix, send SendFunc, d time.Duration, snakeSize, mIdx, mLength int, color packets.LightHsbk) {
	m.Clear()

	pxCache := NewPixelCache(snakeSize)

	var x, y int
	var reversed bool
	for i := range m.Size {
		x, y = nextPixel(m, i, y, &reversed)
		v := i % snakeSize
		if pxCache.IsSet(v) {
			p := pxCache.GetPixel(v)
			m.SetPixel(p.X, p.Y, packets.LightHsbk{})
		}
		pxCache.SetPixel(v, x, y)

		m.SetPixel(x, y, color)
		send(messages.SetMatrixColors(uint8(mIdx), uint8(mLength), uint8(m.Width), m.FlattenColors(), minInterval))
		time.Sleep(d)
	}

	// Clear the tail and turn off all pixels.
	for _, p := range pxCache.Pixels() {
		m.Clear(p)
		send(messages.SetMatrixColors(uint8(mIdx), uint8(mLength), uint8(m.Width), m.FlattenColors(), minInterval))
		time.Sleep(d)
	}
}

// repeatForCycles repeats the given function for n cycles or indefinitely if cycles is 0.
func repeatForCycles(cycles int, f func()) {
	if cycles > 0 {
		for range cycles {
			f()
		}
		return
	} else {
		for {
			f()
		}
	}
}

// nextPixel returns the next pixel position for the given index wrapping to the next row.
// If reversed is set, it is used to determine whether to reverse the direction
// after the end of a row is reached.
func nextPixel(m *Matrix, i, y int, reversed *bool) (int, int) {
	x := i % m.Width
	if i > 0 && x == 0 {
		y++
		if reversed != nil {
			*reversed = !*reversed
		}
	}
	if reversed != nil && *reversed {
		x = m.MaxX() - x
	}
	return x, y
}
