package messages

import (
	"math/rand/v2"
	"testing"
	"time"

	"github.com/alessio-palumbo/lifxlan-go/pkg/protocol"
	"github.com/alessio-palumbo/lifxprotocol-go/gen/protocol/packets"
	"github.com/stretchr/testify/assert"
)

func TestSetMatrixColorsFromSlice(t *testing.T) {
	newNColors := func(n int) []packets.LightHsbk {
		s := make([]packets.LightHsbk, n)
		hue := rand.IntN(360)
		for i := range s {
			s[i] = packets.LightHsbk{Hue: uint16(hue)}
		}
		return s
	}

	lessThan64Slice := newNColors(35)
	lessThan64Array := [64]packets.LightHsbk{}
	copy(lessThan64Array[:], lessThan64Slice)

	equal64Slice := newNColors(64)
	equal64Array := [64]packets.LightHsbk{}
	copy(equal64Array[:], equal64Slice)

	greaterThan64Slice := newNColors(128)
	greaterThan64Array1 := [64]packets.LightHsbk{}
	greaterThan64Array2 := [64]packets.LightHsbk{}
	copy(greaterThan64Array1[:], greaterThan64Slice[:64])
	copy(greaterThan64Array2[:], greaterThan64Slice[64:])

	testCases := map[string]struct {
		startIndex int
		length     int
		width      int
		x, y       int
		colors     []packets.LightHsbk
		d          time.Duration
		want       []*protocol.Message
	}{
		"less than 64 colors": {
			length: 1,
			width:  7,
			colors: lessThan64Slice,
			d:      time.Millisecond,
			want: []*protocol.Message{
				protocol.NewMessage(&packets.TileSet64{
					TileIndex: 0, Length: 1, Rect: packets.TileBufferRect{Width: 7, X: 0, Y: 0},
					Duration: 1, Colors: lessThan64Array,
				}),
			},
		},
		"equal 64 colors": {
			length: 1,
			width:  8,
			colors: equal64Slice,
			d:      time.Millisecond,
			want: []*protocol.Message{
				protocol.NewMessage(&packets.TileSet64{
					TileIndex: 0, Length: 1, Rect: packets.TileBufferRect{Width: 8, X: 0, Y: 0},
					Duration: 1, Colors: equal64Array,
				}),
			},
		},
		"greater than 64 colors": {
			length: 1,
			width:  16,
			colors: greaterThan64Slice,
			d:      time.Millisecond,
			want: []*protocol.Message{
				protocol.NewMessage(&packets.TileSet64{
					TileIndex: 0, Length: 1, Rect: packets.TileBufferRect{Width: 16, X: 0, Y: 0},
					Duration: 1, Colors: greaterThan64Array1,
				}),
				protocol.NewMessage(&packets.TileSet64{
					TileIndex: 0, Length: 1, Rect: packets.TileBufferRect{Width: 16, X: 0, Y: 4},
					Duration: 1, Colors: greaterThan64Array2,
				}),
			},
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			got := SetMatrixColorsFromSlice(tc.startIndex, tc.length, tc.width, tc.colors, tc.d)
			assert.Equal(t, got, tc.want)
		})
	}
}
