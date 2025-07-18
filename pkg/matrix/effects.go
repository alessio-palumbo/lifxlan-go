package matrix

import (
	"fmt"
	"time"

	"github.com/alessio-palumbo/lifxlan-go/pkg/messages"
	"github.com/alessio-palumbo/lifxlan-go/pkg/protocol"
	"github.com/alessio-palumbo/lifxprotocol-go/gen/protocol/packets"
)

var (
	minInterval  = time.Millisecond
	minSnakeSize = 1
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

type SendFunc = func(msg *protocol.Message) error

// Waterfall applies the given colours sequentially on each row centering them, if possible.
// It waits for the given interval before setting the next row.
// It repeats for n cycles, if cycles is set to 0 it repeats indefinitely.
func Waterfall(m *Matrix, send SendFunc, sendIntervalMs int64, cycles int, mode chainMode, colors ...packets.LightHsbk) {
	d := max(time.Duration(sendIntervalMs)*time.Millisecond, minInterval)

	if len(colors) > m.Width {
		colors = colors[:m.Width]
	}
	// Set x so that the colors are centered.
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

func Slug(m *Matrix, send SendFunc, sendIntervalMs int64, size int, color packets.LightHsbk) {
	d := max(time.Duration(sendIntervalMs)*time.Millisecond, minInterval)
	snakeSize := min(max(size, minSnakeSize), m.Width)

	var y int
	var reversed bool
	var pixelsSet int
	for i := range m.Size {
		x := i % m.Width
		if i > 0 && x == 0 {
			y++
			reversed = !reversed
		}
		if reversed {
			x = m.MaxX() - x
		}
		pixelsSet++
		if pixelsSet >= snakeSize {
			m.Clear()
			pixelsSet = 0
		}
		m.SetPixel(x, y, color)
		send(messages.SetMatrixColors(0, 1, uint8(m.Width), m.FlattenColors(), time.Millisecond))
		time.Sleep(d)
	}
}

func Snake(m *Matrix, send SendFunc, sendIntervalMs int64, size int, color packets.LightHsbk) {
	d := max(time.Duration(sendIntervalMs)*time.Millisecond, minInterval)
	snakeSize := min(max(size, minSnakeSize), m.Width)

	var y int
	var reversed bool
	var pixelsSet int
	clearX, clearY := 0, 0
	for i := range m.Size {
		x := i % m.Width
		if i > 0 && x == 0 {
			y++
			reversed = !reversed
		}
		if reversed {
			x = m.MaxX() - x
		}
		pixelsSet++
		if pixelsSet == snakeSize {
			m.SetPixel(clearX, clearY, packets.LightHsbk{})
			fmt.Println("Prev", clearX, clearY, "New", x, y)
			clearX, clearY = x, y
			pixelsSet = 0
		}
		m.SetPixel(x, y, color)
		send(messages.SetMatrixColors(0, 1, uint8(m.Width), m.FlattenColors(), time.Millisecond))
		time.Sleep(d)
	}
}

func Snakes(m *Matrix, send SendFunc, sendIntervalMs int64, color packets.LightHsbk) {
	d := max(time.Duration(sendIntervalMs)*time.Millisecond, minInterval)

	var y int
	var reversed bool
	for i := range m.Size {
		x := i % m.Width
		if i > 0 && x == 0 {
			y++
			reversed = !reversed
		}
		if reversed {
			x = m.MaxX() - x
		}
		m.Clear()
		m.SetPixel(x, y, color)
		send(messages.SetMatrixColors(0, 1, uint8(m.Width), m.FlattenColors(), time.Millisecond))
		time.Sleep(d)
	}
}

// repeatForCycles repeats the given function for n cycles or indefinitely is cycles is 0.
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
