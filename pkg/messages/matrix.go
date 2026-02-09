package messages

import (
	"math/rand"
	"time"

	"github.com/alessio-palumbo/lifxlan-go/pkg/protocol"
	"github.com/alessio-palumbo/lifxprotocol-go/gen/protocol/enums"
	"github.com/alessio-palumbo/lifxprotocol-go/gen/protocol/packets"
)

const (
	defaultCloudsMinSaturation = 50
)

// SetMatrixColors returns a TileSet64 Message that sets a matrix with the given size to the provided colors.
func SetMatrixColors(startIndex, length, width int, colors [64]packets.LightHsbk, d time.Duration) *protocol.Message {
	return newTileSet64Msg(startIndex, length, 0, width, 0, 0, colors, d)
}

// SetMatrixColorsFromSlice returns one or more TileSet64 messages according to a slice of packets.LightHsbk.
// If the slice does not contain all the 64 colors then the default zero value is used.
func SetMatrixColorsFromSlice(startIndex, length, width int, colors []packets.LightHsbk, d time.Duration) []*protocol.Message {
	var msgs []*protocol.Message
	hsbk := [64]packets.LightHsbk{}
	var tileIndex int

	var fb int
	var flipDuration time.Duration
	if len(colors) > 64 {
		fb = 1
		flipDuration = d
		d = 0
	}

	for i, c := range colors {
		hsbk[i%64] = c

		if (i+1)%64 == 0 || i == len(colors)-1 {
			y := tileIndex * 64 / width
			msgs = append(msgs, newTileSet64Msg(startIndex, length, fb, width, 0, y, hsbk, d))
			hsbk = [64]packets.LightHsbk{}
			tileIndex++
		}
	}

	if fb == 1 {
		// Compute height based on the width and length of colors
		height := len(colors) / width
		msgs = append(msgs, SetMatrixVisibleFrameBuffer(startIndex, length, fb, width, height, flipDuration))
	}

	return msgs
}

// SetMatrixEffectOff returns a message instructing the device to turn any running matrix effect off.
func SetMatrixEffectOff() *protocol.Message {
	return protocol.NewMessage(&packets.TileSetEffect{
		Settings: packets.TileEffectSettings{
			Instanceid: rand.Uint32(),
			Type:       enums.TileEffectTypeTILEEFFECTTYPEOFF,
		},
	})
}

// SetMatrixFlameEffect returns a message instructing the device to run the Flame effect.
func SetMatrixFlameEffect(speed time.Duration) *protocol.Message {
	return protocol.NewMessage(&packets.TileSetEffect{
		Settings: packets.TileEffectSettings{
			Instanceid: rand.Uint32(),
			Type:       enums.TileEffectTypeTILEEFFECTTYPEFLAME,
			Speed:      uint32(speed.Milliseconds()),
		},
	})
}

// SetMatrixMorphEffect returns a message instructing the device to run the Morph effect.
// A palette of up to 16 colors is used for the transition.
func SetMatrixMorphEffect(speed time.Duration, colors ...packets.LightHsbk) *protocol.Message {
	if len(colors) > 16 {
		colors = colors[:16]
	}
	var palette [16]packets.LightHsbk
	copy(palette[:], colors)

	return protocol.NewMessage(&packets.TileSetEffect{
		Settings: packets.TileEffectSettings{
			Instanceid:   rand.Uint32(),
			Type:         enums.TileEffectTypeTILEEFFECTTYPEMORPH,
			PaletteCount: uint8(len(colors)),
			Palette:      palette,
			Speed:        uint32(speed.Milliseconds()),
		},
	})
}

// SetMatrixCloudsEffect returns a message instructing the device to run the Cloud effect.
// If minSaturation (0-255) is not set a default value is used.
func SetMatrixCloudsEffect(speed time.Duration, minSaturation *uint32) *protocol.Message {
	if minSaturation != nil {
		*minSaturation = min(*minSaturation, 100)
	} else {
		minSaturation = new(uint32)
		*minSaturation = uint32(defaultCloudsMinSaturation)
	}
	return protocol.NewMessage(&packets.TileSetEffect{
		Settings: packets.TileEffectSettings{
			Instanceid: rand.Uint32(),
			Type:       enums.TileEffectTypeTILEEFFECTTYPESKY,
			Parameter: packets.TileEffectParameter{
				Parameter0: uint32(enums.TileEffectSkyTypeTILEEFFECTSKYTYPECLOUDS),
				Parameter1: *minSaturation,
			},
			Speed: uint32(speed.Milliseconds()),
		},
	})
}

// SetMatrixSunriseEffect returns a message instructing the device to run the Sunrise effect.
func SetMatrixSunriseEffect(speed *time.Duration) *protocol.Message {
	if speed == nil {
		speed = new(time.Duration)
	}
	return protocol.NewMessage(&packets.TileSetEffect{
		Settings: packets.TileEffectSettings{
			Instanceid: rand.Uint32(),
			Type:       enums.TileEffectTypeTILEEFFECTTYPESKY,
			Parameter: packets.TileEffectParameter{
				Parameter0: uint32(enums.TileEffectSkyTypeTILEEFFECTSKYTYPESUNRISE),
			},
			Speed: uint32(speed.Milliseconds()),
		},
	})
}

// SetMatrixSunsetEffect returns a message instructing the device to run the Sunset effect.
// If softOff is set to true the device will turn off at the end of the effect.
func SetMatrixSunsetEffect(speed *time.Duration, softOff bool) *protocol.Message {
	if speed == nil {
		speed = new(time.Duration)
	}
	var p0 uint32
	if softOff {
		p0 = 1
	}
	return protocol.NewMessage(&packets.TileSetEffect{
		Settings: packets.TileEffectSettings{
			Instanceid: rand.Uint32(),
			Type:       enums.TileEffectTypeTILEEFFECTTYPESKY,
			Parameter: packets.TileEffectParameter{
				Parameter0: uint32(enums.TileEffectSkyTypeTILEEFFECTSKYTYPESUNSET),
				Parameter1: p0,
			},
			Speed: uint32(speed.Milliseconds()),
		},
	})
}

// SetMatrixFrameAnimation returns a slice of messages to preloads animation frames into device hidden frame buffers
// and a function the returns a message that copies the next frame into the visible buffer.
func SetMatrixFrameAnimation(startIndex, length, width int, frames [][]packets.LightHsbk, brightness float64, d time.Duration) ([]*protocol.Message, func() *protocol.Message) {
	frameCount := len(frames)
	if frameCount == 0 {
		return nil, nil
	}

	var msgs []*protocol.Message
	brightness = max(1, min(brightness, 100))

	// Load each frame into fb 1..N
	for fb := range frameCount {
		colors := frames[fb]

		hsbk := [64]packets.LightHsbk{}
		tileIndex := 0

		for i, c := range colors {
			c.Brightness = uint16(float64(c.Brightness) / 100 * brightness)
			hsbk[i%64] = c

			if (i+1)%64 == 0 || i == len(colors)-1 {
				y := tileIndex * 64 / width
				msgs = append(msgs, newTileSet64Msg(startIndex, length, fb+1, width, 0, y, hsbk, 0))
				hsbk = [64]packets.LightHsbk{}
				tileIndex++
			}
		}
	}

	// activeFrame is the index of the last frame copied into the visible buffer (0).
	var activeFrame int

	return msgs, func() *protocol.Message {
		// nextFrameFb is the frame buffer that will be copied into the visible frame buffer (0).
		nextFrameFb := activeFrame + 1
		activeFrame = (nextFrameFb) % frameCount
		return SetMatrixVisibleFrameBuffer(startIndex, length, nextFrameFb, width, len(frames[0])/width, d)
	}
}

// SetMatrixVisibleFrameBuffer copies the given frame buffer (fb) into the visible frame buffer (0).
// This can be used to switch between previously stored frame buffers for animations or smooth transitions (as in the case
// of matrix that exceeds 64 colors and therefore needs multiple messages to be set).
func SetMatrixVisibleFrameBuffer(startIndex, length, fb, width, height int, d time.Duration) *protocol.Message {
	return protocol.NewMessage(&packets.TileCopyFrameBuffer{
		TileIndex:  uint8(startIndex),
		Length:     uint8(length),
		DstFbIndex: 0, // Visible buffer
		SrcFbIndex: uint8(fb),
		Width:      uint8(width),
		Height:     uint8(height),
		Duration:   uint32(d.Milliseconds()),
	})
}

func newTileSet64Msg(startIndex, length, fb, width, x, y int, colors [64]packets.LightHsbk, d time.Duration) *protocol.Message {
	m := &packets.TileSet64{
		TileIndex: uint8(startIndex),
		Length:    uint8(length),
		Rect:      packets.TileBufferRect{FbIndex: uint8(fb), Width: uint8(width), X: uint8(x), Y: uint8(y)},
		Duration:  uint32(d.Milliseconds()),
		Colors:    colors,
	}
	return protocol.NewMessage(m)
}
