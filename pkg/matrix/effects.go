package matrix

import (
	"fmt"
	"math"
	"math/rand/v2"
	"sync/atomic"
	"time"

	"github.com/alessio-palumbo/lifxlan-go/pkg/iterator"
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

// ParseChainMode converts an int to chainmode.
// If invalid it return ChainModeNone.
func ParseChainMode(m int) chainMode {
	switch m {
	case 1:
		return ChainModeNone
	case 2:
		return ChainModeSequential
	default:
		return ChainModeNone
	}
}

type animationDirection int

const (
	AnimationDirectionInwards animationDirection = iota
	AnimationDirectionOutwards
	AnimationDirectionInOut
	AnimationDirectionOutIn
)

// ParseAnimationDirection converts an int to animationDirection.
// If invalid it return AnimationDirectionInwards.
func ParseAnimationDirection(m int) animationDirection {
	switch m {
	case 1:
		return AnimationDirectionOutwards
	case 2:
		return AnimationDirectionInOut
	case 3:
		return AnimationDirectionInOut
	default:
		return AnimationDirectionInwards
	}
}

// SendFunc is an interface for sending protocol messages.
type SendFunc = func(msg *protocol.Message) error

// ErrStopped is the error returned when manually stopping an effect.
var ErrStopped = fmt.Errorf("manual stop")

// SendWithStop wraps a SendFunc with an atomic.Bool to manually stop the sender
// at the next cycle.
func SendWithStop(send SendFunc) (SendFunc, *atomic.Bool) {
	var stopped atomic.Bool
	return func(msg *protocol.Message) error {
		if stopped.Load() {
			return ErrStopped
		}
		return send(msg)
	}, &stopped
}

// Waterfall applies the given colors sequentially on each row centering them, if possible.
// It waits for the given interval before setting the next row.
// It repeats for n cycles, if cycles is set to 0 it repeats indefinitely.
func Waterfall(m *Matrix, send SendFunc, sendIntervalMs int64, cycles int, mode chainMode, colors ...packets.LightHsbk) error {
	d := max(time.Duration(sendIntervalMs)*time.Millisecond, minInterval)

	if len(colors) > m.Width {
		colors = colors[:m.Width]
	}
	// Try to center the colors if possible.
	x := (m.Width - len(colors)) / 2

	return repeatForCycles(cycles, func() error {
		switch mode {
		case ChainModeSequential:
			for ti := range m.ChainLength {
				if err := waterfall(m, send, d, x, ti, 1, colors...); err != nil {
					return err
				}
			}
			return nil
		case ChainModeSynced:
			return waterfall(m, send, d, x, 0, m.ChainLength, colors...)
		default:
			return waterfall(m, send, d, x, 0, 1, colors...)
		}
	})
}

func waterfall(m *Matrix, send SendFunc, d time.Duration, x, mIdx, mLength int, colors ...packets.LightHsbk) error {
	m.Clear()

	for i := range m.Height {
		m.SetColors(x, i, colors...)
		if err := send(messages.SetMatrixColors(uint8(mIdx), uint8(mLength), uint8(m.Width), m.FlattenColors(), minInterval)); err != nil {
			return err
		}
		time.Sleep(d)
	}
	return nil
}

// Rockets sequentially applies a color to a single pixel row by row wrapping at the end.
// It waits for the given interval before setting the next pixel.
// If multiple colors are provided colors are rotated on each row.
// It repeats for n cycles, if cycles is set to 0 it repeats indefinitely.
func Rockets(m *Matrix, send SendFunc, sendIntervalMs int64, cycles int, mode chainMode, colors ...packets.LightHsbk) error {
	d := max(time.Duration(sendIntervalMs)*time.Millisecond, minInterval)

	return repeatForCycles(cycles, func() error {
		switch mode {
		case ChainModeSequential:
			for ti := range m.ChainLength {
				if err := rockets(m, send, d, ti, 1, colors...); err != nil {
					return err
				}
			}
			return nil
		case ChainModeSynced:
			return rockets(m, send, d, 0, m.ChainLength, colors...)
		default:
			return rockets(m, send, d, 0, 1, colors...)
		}
	})
}

func rockets(m *Matrix, send SendFunc, d time.Duration, mIdx, mLength int, colors ...packets.LightHsbk) error {
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
		if err := send(messages.SetMatrixColors(uint8(mIdx), uint8(mLength), uint8(m.Width), m.FlattenColors(), minInterval)); err != nil {
			return err
		}
		time.Sleep(d)
	}
	return nil
}

// Worm moves n pixels along the matrix wrapping and reversing at every row with a wave-like movement.
// It repeats for n cycles, if cycles is set to 0 it repeats indefinitely.
func Worm(m *Matrix, send SendFunc, sendIntervalMs int64, cycles int, mode chainMode, size int, color packets.LightHsbk) error {
	d := max(time.Duration(sendIntervalMs)*time.Millisecond, minInterval)
	wormSize := min(max(size, 1), m.Width)

	return repeatForCycles(cycles, func() error {
		switch mode {
		case ChainModeSequential:
			for ti := range m.ChainLength {
				if err := worm(m, send, d, wormSize, ti, 1, color); err != nil {
					return err
				}
			}
			return nil
		case ChainModeSynced:
			return worm(m, send, d, wormSize, 0, m.ChainLength, color)
		default:
			return worm(m, send, d, wormSize, 0, 1, color)
		}
	})
}

func worm(m *Matrix, send SendFunc, d time.Duration, wormSize, mIdx, mLength int, color packets.LightHsbk) error {
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
		if err := send(messages.SetMatrixColors(uint8(mIdx), uint8(mLength), uint8(m.Width), m.FlattenColors(), minInterval)); err != nil {
			return err
		}
		time.Sleep(d)
	}

	// Clear the tail and turn off all pixels.
	for _, p := range pxCache.Pixels() {
		m.Clear(p)
		if err := send(messages.SetMatrixColors(uint8(mIdx), uint8(mLength), uint8(m.Width), m.FlattenColors(), minInterval)); err != nil {
			return err
		}
		time.Sleep(d)
	}
	return nil
}

// Snake moves n pixels along the matrix wrapping and reversing at every row.
// It repeats for n cycles, if cycles is set to 0 it repeats indefinitely.
func Snake(m *Matrix, send SendFunc, sendIntervalMs int64, cycles int, mode chainMode, size int, color packets.LightHsbk) error {
	d := max(time.Duration(sendIntervalMs)*time.Millisecond, minInterval)
	snakeSize := min(max(size, 1), m.Width)

	return repeatForCycles(cycles, func() error {
		switch mode {
		case ChainModeSequential:
			for ti := range m.ChainLength {
				if err := snake(m, send, d, snakeSize, ti, 1, color); err != nil {
					return err
				}
			}
			return nil
		case ChainModeSynced:
			return snake(m, send, d, snakeSize, 0, m.ChainLength, color)
		default:
			return snake(m, send, d, snakeSize, 0, 1, color)
		}
	})
}

func snake(m *Matrix, send SendFunc, d time.Duration, snakeSize, mIdx, mLength int, color packets.LightHsbk) error {
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
		if err := send(messages.SetMatrixColors(uint8(mIdx), uint8(mLength), uint8(m.Width), m.FlattenColors(), minInterval)); err != nil {
			return nil
		}
		time.Sleep(d)
	}

	// Clear the tail and turn off all pixels.
	for _, p := range pxCache.Pixels() {
		m.Clear(p)
		if err := send(messages.SetMatrixColors(uint8(mIdx), uint8(mLength), uint8(m.Width), m.FlattenColors(), minInterval)); err != nil {
			return err
		}
		time.Sleep(d)
	}
	return nil
}

// ConcentricFrames iterates according to the given direction drawing frams of variadic sizes.
// If an optional color is supplied it is used as the fram color, otherwise the frame color
// will randomly change at each iteration.
// It repeats for n cycles, if cycles is set to 0 it repeats indefinitely.
func ConcentricFrames(m *Matrix, send SendFunc, sendIntervalMs int64, cycles int, mode chainMode, direction animationDirection, color *packets.LightHsbk) error {
	d := max(time.Duration(sendIntervalMs)*time.Millisecond, minInterval)
	var iterFunc func(yield func(int) bool)
	maxSteps := m.MaxPadding() + 1

	switch direction {
	case AnimationDirectionInwards:
		iterFunc = iterator.IterateUp(0, maxSteps)
	case AnimationDirectionOutwards:
		iterFunc = iterator.IterateDown(maxSteps, 0)
	case AnimationDirectionInOut:
		iterFunc = iterator.BounceUp(maxSteps)
	case AnimationDirectionOutIn:
		iterFunc = iterator.BounceDown(maxSteps)
	}

	return repeatForCycles(cycles, func() error {
		switch mode {
		case ChainModeSequential:
			for ti := range m.ChainLength {
				if err := concentricFrames(m, send, d, ti, 1, iterFunc, color); err != nil {
					return err
				}
			}
			return nil
		case ChainModeSynced:
			return concentricFrames(m, send, d, 0, m.ChainLength, iterFunc, color)
		default:
			return concentricFrames(m, send, d, 0, 1, iterFunc, color)
		}
	})
}

func concentricFrames(m *Matrix, send SendFunc, d time.Duration, mIdx, mLength int, iterator func(yield func(int) bool), color *packets.LightHsbk) error {
	m.Clear()

	if color == nil {
		color = &packets.LightHsbk{
			Hue:        uint16(rand.UintN(math.MaxUint16)),
			Saturation: 65535,
			Brightness: 65535,
			Kelvin:     3500,
		}
	}

	for p := range iterator {
		m.Clear()
		m.SetBorder(p, *color)
		if err := send(messages.SetMatrixColors(uint8(mIdx), uint8(mLength), uint8(m.Width), m.FlattenColors(), minInterval)); err != nil {
			return err
		}
		time.Sleep(d)
	}

	return nil
}

// repeatForCycles repeats the given function for n cycles or indefinitely if cycles is 0.
func repeatForCycles(cycles int, f func() error) error {
	if cycles > 0 {
		for range cycles {
			if err := f(); err != nil {
				return err
			}
		}
		return nil
	} else {
		for {
			if err := f(); err != nil {
				return err
			}
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
