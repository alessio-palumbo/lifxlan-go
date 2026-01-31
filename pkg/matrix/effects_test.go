package matrix

import (
	"testing"
	"time"

	"github.com/alessio-palumbo/lifxlan-go/pkg/protocol"
	"github.com/alessio-palumbo/lifxprotocol-go/gen/protocol/packets"
	"github.com/stretchr/testify/assert"
)

func TestParseChainMode(t *testing.T) {
	testCases := map[string]struct {
		value int
		want  ChainMode
	}{
		"mode none": {
			value: 0, want: ChainModeNone,
		},
		"mode sequential": {
			value: 1, want: ChainModeSequential,
		},
		"mode synced": {
			value: 2, want: ChainModeSynced,
		},
		"default to mode none": {
			value: 100, want: ChainModeNone,
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			if got := ParseChainMode(tc.value); got != tc.want {
				t.Fatalf("Expected %v, got %v", tc.want, got)
			}
		})
	}
}

func TestParseAnimationDirection(t *testing.T) {
	testCases := map[string]struct {
		value int
		want  AnimationDirection
	}{
		"direction inwards": {
			value: 0, want: AnimationDirectionInwards,
		},
		"direction outwards": {
			value: 1, want: AnimationDirectionOutwards,
		},
		"direction in-out": {
			value: 2, want: AnimationDirectionInOut,
		},
		"direction out-in": {
			value: 3, want: AnimationDirectionOutIn,
		},
		"default to direction inwards": {
			value: 100, want: AnimationDirectionInwards,
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			if got := ParseAnimationDirection(tc.value); got != tc.want {
				t.Fatalf("Expected %v, got %v", tc.want, got)
			}
		})
	}
}

func TestSendWithStop(t *testing.T) {
	f := func(msg *protocol.Message) error {
		return nil
	}
	done := make(chan struct{})

	wrapF, stopped := SendWithStop(f)
	go func() {
		for {
			if err := wrapF(nil); err != nil {
				close(done)
				return
			}
		}
	}()

	stopped.Store(true)

	select {
	case <-done:
	case <-time.After(10 * time.Millisecond):
		t.Fatal("timed out")
	}
}

func TestWaterfall(t *testing.T) {
	testCases := map[string]struct {
		mode    ChainMode
		matrix  *Matrix
		colors  []packets.LightHsbk
		want    []packets.Payload
		wantErr error
	}{
		"missing colors": {
			wantErr: ErrMissingColors,
		},
		"single tile": {
			matrix: New(4, 4, 2),
			colors: []packets.LightHsbk{{Kelvin: 3500}, {Kelvin: 3600}},
			want: []packets.Payload{
				&packets.TileSet64{
					TileIndex: 0, Length: 1, Rect: packets.TileBufferRect{Width: 4}, Duration: 1,
					Colors: [64]packets.LightHsbk{
						{}, {Kelvin: 3500}, {Kelvin: 3600}, {},
					},
				},
				&packets.TileSet64{
					TileIndex: 0, Length: 1, Rect: packets.TileBufferRect{Width: 4}, Duration: 1,
					Colors: [64]packets.LightHsbk{
						{}, {Kelvin: 3500}, {Kelvin: 3600}, {},
						{}, {Kelvin: 3500}, {Kelvin: 3600}, {},
					},
				},
				&packets.TileSet64{
					TileIndex: 0, Length: 1, Rect: packets.TileBufferRect{Width: 4}, Duration: 1,
					Colors: [64]packets.LightHsbk{
						{}, {Kelvin: 3500}, {Kelvin: 3600}, {},
						{}, {Kelvin: 3500}, {Kelvin: 3600}, {},
						{}, {Kelvin: 3500}, {Kelvin: 3600}, {},
					},
				},
				&packets.TileSet64{
					TileIndex: 0, Length: 1, Rect: packets.TileBufferRect{Width: 4}, Duration: 1,
					Colors: [64]packets.LightHsbk{
						{}, {Kelvin: 3500}, {Kelvin: 3600}, {},
						{}, {Kelvin: 3500}, {Kelvin: 3600}, {},
						{}, {Kelvin: 3500}, {Kelvin: 3600}, {},
						{}, {Kelvin: 3500}, {Kelvin: 3600}, {},
					},
				},
			},
		},
		"multiple tiles: sequential": {
			mode:   ChainModeSequential,
			matrix: New(4, 4, 2),
			colors: []packets.LightHsbk{{Kelvin: 3500}, {Kelvin: 3600}},
			want: []packets.Payload{
				&packets.TileSet64{
					TileIndex: 0, Length: 1, Rect: packets.TileBufferRect{Width: 4}, Duration: 1,
					Colors: [64]packets.LightHsbk{
						{}, {Kelvin: 3500}, {Kelvin: 3600}, {},
					},
				},
				&packets.TileSet64{
					TileIndex: 0, Length: 1, Rect: packets.TileBufferRect{Width: 4}, Duration: 1,
					Colors: [64]packets.LightHsbk{
						{}, {Kelvin: 3500}, {Kelvin: 3600}, {},
						{}, {Kelvin: 3500}, {Kelvin: 3600}, {},
					},
				},
				&packets.TileSet64{
					TileIndex: 0, Length: 1, Rect: packets.TileBufferRect{Width: 4}, Duration: 1,
					Colors: [64]packets.LightHsbk{
						{}, {Kelvin: 3500}, {Kelvin: 3600}, {},
						{}, {Kelvin: 3500}, {Kelvin: 3600}, {},
						{}, {Kelvin: 3500}, {Kelvin: 3600}, {},
					},
				},
				&packets.TileSet64{
					TileIndex: 0, Length: 1, Rect: packets.TileBufferRect{Width: 4}, Duration: 1,
					Colors: [64]packets.LightHsbk{
						{}, {Kelvin: 3500}, {Kelvin: 3600}, {},
						{}, {Kelvin: 3500}, {Kelvin: 3600}, {},
						{}, {Kelvin: 3500}, {Kelvin: 3600}, {},
						{}, {Kelvin: 3500}, {Kelvin: 3600}, {},
					},
				},
				&packets.TileSet64{
					TileIndex: 1, Length: 1, Rect: packets.TileBufferRect{Width: 4}, Duration: 1,
					Colors: [64]packets.LightHsbk{
						{}, {Kelvin: 3500}, {Kelvin: 3600}, {},
					},
				},
				&packets.TileSet64{
					TileIndex: 1, Length: 1, Rect: packets.TileBufferRect{Width: 4}, Duration: 1,
					Colors: [64]packets.LightHsbk{
						{}, {Kelvin: 3500}, {Kelvin: 3600}, {},
						{}, {Kelvin: 3500}, {Kelvin: 3600}, {},
					},
				},
				&packets.TileSet64{
					TileIndex: 1, Length: 1, Rect: packets.TileBufferRect{Width: 4}, Duration: 1,
					Colors: [64]packets.LightHsbk{
						{}, {Kelvin: 3500}, {Kelvin: 3600}, {},
						{}, {Kelvin: 3500}, {Kelvin: 3600}, {},
						{}, {Kelvin: 3500}, {Kelvin: 3600}, {},
					},
				},
				&packets.TileSet64{
					TileIndex: 1, Length: 1, Rect: packets.TileBufferRect{Width: 4}, Duration: 1,
					Colors: [64]packets.LightHsbk{
						{}, {Kelvin: 3500}, {Kelvin: 3600}, {},
						{}, {Kelvin: 3500}, {Kelvin: 3600}, {},
						{}, {Kelvin: 3500}, {Kelvin: 3600}, {},
						{}, {Kelvin: 3500}, {Kelvin: 3600}, {},
					},
				},
			},
		},
		"multiple tiles: synced": {
			mode:   ChainModeSynced,
			matrix: New(4, 4, 2),
			colors: []packets.LightHsbk{{Kelvin: 3500}, {Kelvin: 3600}},
			want: []packets.Payload{
				&packets.TileSet64{
					TileIndex: 0, Length: 2, Rect: packets.TileBufferRect{Width: 4}, Duration: 1,
					Colors: [64]packets.LightHsbk{
						{}, {Kelvin: 3500}, {Kelvin: 3600}, {},
					},
				},
				&packets.TileSet64{
					TileIndex: 0, Length: 2, Rect: packets.TileBufferRect{Width: 4}, Duration: 1,
					Colors: [64]packets.LightHsbk{
						{}, {Kelvin: 3500}, {Kelvin: 3600}, {},
						{}, {Kelvin: 3500}, {Kelvin: 3600}, {},
					},
				},
				&packets.TileSet64{
					TileIndex: 0, Length: 2, Rect: packets.TileBufferRect{Width: 4}, Duration: 1,
					Colors: [64]packets.LightHsbk{
						{}, {Kelvin: 3500}, {Kelvin: 3600}, {},
						{}, {Kelvin: 3500}, {Kelvin: 3600}, {},
						{}, {Kelvin: 3500}, {Kelvin: 3600}, {},
					},
				},
				&packets.TileSet64{
					TileIndex: 0, Length: 2, Rect: packets.TileBufferRect{Width: 4}, Duration: 1,
					Colors: [64]packets.LightHsbk{
						{}, {Kelvin: 3500}, {Kelvin: 3600}, {},
						{}, {Kelvin: 3500}, {Kelvin: 3600}, {},
						{}, {Kelvin: 3500}, {Kelvin: 3600}, {},
						{}, {Kelvin: 3500}, {Kelvin: 3600}, {},
					},
				},
			},
		},
		"matrix greater than 64": {
			matrix: New(16, 8, 1),
			colors: []packets.LightHsbk{{Kelvin: 3500}, {Kelvin: 3600}},
			want: []packets.Payload{
				&packets.TileSet64{
					TileIndex: 0, Length: 1, Rect: packets.TileBufferRect{Width: 16, FbIndex: 1, Y: 0},
					Colors: [64]packets.LightHsbk{
						{}, {}, {}, {}, {}, {}, {}, {Kelvin: 3500}, {Kelvin: 3600}, {}, {}, {}, {}, {}, {}, {},
					},
				},
				&packets.TileSet64{
					TileIndex: 0, Length: 1, Rect: packets.TileBufferRect{Width: 16, FbIndex: 1, Y: 4},
					Colors: [64]packets.LightHsbk{},
				},
				&packets.TileCopyFrameBuffer{
					DstFbIndex: 0, SrcFbIndex: 1, Width: uint8(16),
					Height: uint8(8), Length: 1, Duration: 1,
				},
				&packets.TileSet64{
					TileIndex: 0, Length: 1, Rect: packets.TileBufferRect{Width: 16, FbIndex: 1, Y: 0},
					Colors: [64]packets.LightHsbk{
						{}, {}, {}, {}, {}, {}, {}, {Kelvin: 3500}, {Kelvin: 3600}, {}, {}, {}, {}, {}, {}, {},
						{}, {}, {}, {}, {}, {}, {}, {Kelvin: 3500}, {Kelvin: 3600}, {}, {}, {}, {}, {}, {}, {},
					},
				},
				&packets.TileSet64{
					TileIndex: 0, Length: 1, Rect: packets.TileBufferRect{Width: 16, FbIndex: 1, Y: 4},
					Colors: [64]packets.LightHsbk{},
				},
				&packets.TileCopyFrameBuffer{
					DstFbIndex: 0, SrcFbIndex: 1, Width: uint8(16),
					Height: uint8(8), Length: 1, Duration: 1,
				},
				&packets.TileSet64{
					TileIndex: 0, Length: 1, Rect: packets.TileBufferRect{Width: 16, FbIndex: 1, Y: 0},
					Colors: [64]packets.LightHsbk{
						{}, {}, {}, {}, {}, {}, {}, {Kelvin: 3500}, {Kelvin: 3600}, {}, {}, {}, {}, {}, {}, {},
						{}, {}, {}, {}, {}, {}, {}, {Kelvin: 3500}, {Kelvin: 3600}, {}, {}, {}, {}, {}, {}, {},
						{}, {}, {}, {}, {}, {}, {}, {Kelvin: 3500}, {Kelvin: 3600}, {}, {}, {}, {}, {}, {}, {},
					},
				},
				&packets.TileSet64{
					TileIndex: 0, Length: 1, Rect: packets.TileBufferRect{Width: 16, FbIndex: 1, Y: 4},
					Colors: [64]packets.LightHsbk{},
				},
				&packets.TileCopyFrameBuffer{
					DstFbIndex: 0, SrcFbIndex: 1, Width: uint8(16),
					Height: uint8(8), Length: 1, Duration: 1,
				},
				&packets.TileSet64{
					TileIndex: 0, Length: 1, Rect: packets.TileBufferRect{Width: 16, FbIndex: 1, Y: 0},
					Colors: [64]packets.LightHsbk{
						{}, {}, {}, {}, {}, {}, {}, {Kelvin: 3500}, {Kelvin: 3600}, {}, {}, {}, {}, {}, {}, {},
						{}, {}, {}, {}, {}, {}, {}, {Kelvin: 3500}, {Kelvin: 3600}, {}, {}, {}, {}, {}, {}, {},
						{}, {}, {}, {}, {}, {}, {}, {Kelvin: 3500}, {Kelvin: 3600}, {}, {}, {}, {}, {}, {}, {},
						{}, {}, {}, {}, {}, {}, {}, {Kelvin: 3500}, {Kelvin: 3600}, {}, {}, {}, {}, {}, {}, {},
					},
				},
				&packets.TileSet64{
					TileIndex: 0, Length: 1, Rect: packets.TileBufferRect{Width: 16, FbIndex: 1, Y: 4},
					Colors: [64]packets.LightHsbk{},
				},
				&packets.TileCopyFrameBuffer{
					DstFbIndex: 0, SrcFbIndex: 1, Width: uint8(16),
					Height: uint8(8), Length: 1, Duration: 1,
				},
				&packets.TileSet64{
					TileIndex: 0, Length: 1, Rect: packets.TileBufferRect{Width: 16, FbIndex: 1, Y: 0},
					Colors: [64]packets.LightHsbk{
						{}, {}, {}, {}, {}, {}, {}, {Kelvin: 3500}, {Kelvin: 3600}, {}, {}, {}, {}, {}, {}, {},
						{}, {}, {}, {}, {}, {}, {}, {Kelvin: 3500}, {Kelvin: 3600}, {}, {}, {}, {}, {}, {}, {},
						{}, {}, {}, {}, {}, {}, {}, {Kelvin: 3500}, {Kelvin: 3600}, {}, {}, {}, {}, {}, {}, {},
						{}, {}, {}, {}, {}, {}, {}, {Kelvin: 3500}, {Kelvin: 3600}, {}, {}, {}, {}, {}, {}, {},
					},
				},
				&packets.TileSet64{
					TileIndex: 0, Length: 1, Rect: packets.TileBufferRect{Width: 16, FbIndex: 1, Y: 4},
					Colors: [64]packets.LightHsbk{
						{}, {}, {}, {}, {}, {}, {}, {Kelvin: 3500}, {Kelvin: 3600}, {}, {}, {}, {}, {}, {}, {},
					},
				},
				&packets.TileCopyFrameBuffer{
					DstFbIndex: 0, SrcFbIndex: 1, Width: uint8(16),
					Height: uint8(8), Length: 1, Duration: 1,
				},
				&packets.TileSet64{
					TileIndex: 0, Length: 1, Rect: packets.TileBufferRect{Width: 16, FbIndex: 1, Y: 0},
					Colors: [64]packets.LightHsbk{
						{}, {}, {}, {}, {}, {}, {}, {Kelvin: 3500}, {Kelvin: 3600}, {}, {}, {}, {}, {}, {}, {},
						{}, {}, {}, {}, {}, {}, {}, {Kelvin: 3500}, {Kelvin: 3600}, {}, {}, {}, {}, {}, {}, {},
						{}, {}, {}, {}, {}, {}, {}, {Kelvin: 3500}, {Kelvin: 3600}, {}, {}, {}, {}, {}, {}, {},
						{}, {}, {}, {}, {}, {}, {}, {Kelvin: 3500}, {Kelvin: 3600}, {}, {}, {}, {}, {}, {}, {},
					},
				},
				&packets.TileSet64{
					TileIndex: 0, Length: 1, Rect: packets.TileBufferRect{Width: 16, FbIndex: 1, Y: 4},
					Colors: [64]packets.LightHsbk{
						{}, {}, {}, {}, {}, {}, {}, {Kelvin: 3500}, {Kelvin: 3600}, {}, {}, {}, {}, {}, {}, {},
						{}, {}, {}, {}, {}, {}, {}, {Kelvin: 3500}, {Kelvin: 3600}, {}, {}, {}, {}, {}, {}, {},
					},
				},
				&packets.TileCopyFrameBuffer{
					DstFbIndex: 0, SrcFbIndex: 1, Width: uint8(16),
					Height: uint8(8), Length: 1, Duration: 1,
				},
				&packets.TileSet64{
					TileIndex: 0, Length: 1, Rect: packets.TileBufferRect{Width: 16, FbIndex: 1, Y: 0},
					Colors: [64]packets.LightHsbk{
						{}, {}, {}, {}, {}, {}, {}, {Kelvin: 3500}, {Kelvin: 3600}, {}, {}, {}, {}, {}, {}, {},
						{}, {}, {}, {}, {}, {}, {}, {Kelvin: 3500}, {Kelvin: 3600}, {}, {}, {}, {}, {}, {}, {},
						{}, {}, {}, {}, {}, {}, {}, {Kelvin: 3500}, {Kelvin: 3600}, {}, {}, {}, {}, {}, {}, {},
						{}, {}, {}, {}, {}, {}, {}, {Kelvin: 3500}, {Kelvin: 3600}, {}, {}, {}, {}, {}, {}, {},
					},
				},
				&packets.TileSet64{
					TileIndex: 0, Length: 1, Rect: packets.TileBufferRect{Width: 16, FbIndex: 1, Y: 4},
					Colors: [64]packets.LightHsbk{
						{}, {}, {}, {}, {}, {}, {}, {Kelvin: 3500}, {Kelvin: 3600}, {}, {}, {}, {}, {}, {}, {},
						{}, {}, {}, {}, {}, {}, {}, {Kelvin: 3500}, {Kelvin: 3600}, {}, {}, {}, {}, {}, {}, {},
						{}, {}, {}, {}, {}, {}, {}, {Kelvin: 3500}, {Kelvin: 3600}, {}, {}, {}, {}, {}, {}, {},
					},
				},
				&packets.TileCopyFrameBuffer{
					DstFbIndex: 0, SrcFbIndex: 1, Width: uint8(16),
					Height: uint8(8), Length: 1, Duration: 1,
				},
				&packets.TileSet64{
					TileIndex: 0, Length: 1, Rect: packets.TileBufferRect{Width: 16, FbIndex: 1, Y: 0},
					Colors: [64]packets.LightHsbk{
						{}, {}, {}, {}, {}, {}, {}, {Kelvin: 3500}, {Kelvin: 3600}, {}, {}, {}, {}, {}, {}, {},
						{}, {}, {}, {}, {}, {}, {}, {Kelvin: 3500}, {Kelvin: 3600}, {}, {}, {}, {}, {}, {}, {},
						{}, {}, {}, {}, {}, {}, {}, {Kelvin: 3500}, {Kelvin: 3600}, {}, {}, {}, {}, {}, {}, {},
						{}, {}, {}, {}, {}, {}, {}, {Kelvin: 3500}, {Kelvin: 3600}, {}, {}, {}, {}, {}, {}, {},
					},
				},
				&packets.TileSet64{
					TileIndex: 0, Length: 1, Rect: packets.TileBufferRect{Width: 16, FbIndex: 1, Y: 4},
					Colors: [64]packets.LightHsbk{
						{}, {}, {}, {}, {}, {}, {}, {Kelvin: 3500}, {Kelvin: 3600}, {}, {}, {}, {}, {}, {}, {},
						{}, {}, {}, {}, {}, {}, {}, {Kelvin: 3500}, {Kelvin: 3600}, {}, {}, {}, {}, {}, {}, {},
						{}, {}, {}, {}, {}, {}, {}, {Kelvin: 3500}, {Kelvin: 3600}, {}, {}, {}, {}, {}, {}, {},
						{}, {}, {}, {}, {}, {}, {}, {Kelvin: 3500}, {Kelvin: 3600}, {}, {}, {}, {}, {}, {}, {},
					},
				},
				&packets.TileCopyFrameBuffer{
					DstFbIndex: 0, SrcFbIndex: 1, Width: uint8(16),
					Height: uint8(8), Length: 1, Duration: 1,
				},
			},
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			var got []packets.Payload
			send := func(msg *protocol.Message) error {
				got = append(got, msg.Payload)
				return nil
			}
			if err := Waterfall(tc.matrix, send, 1, 1, tc.mode, tc.colors...); err != tc.wantErr {
				t.Fatalf("Got error %v, want %v", err, tc.wantErr)
			}
			assert.Equal(t, got, tc.want)
		})
	}
}

func TestRockets(t *testing.T) {
	testCases := map[string]struct {
		mode       ChainMode
		matrix     *Matrix
		colors     []packets.LightHsbk
		testSubset func(t *testing.T, got, want []packets.Payload)
		want       []packets.Payload
		wantErr    error
	}{
		"missing colors": {
			wantErr: ErrMissingColors,
		},
		"single tile": {
			matrix: New(2, 2, 2),
			colors: []packets.LightHsbk{{Kelvin: 3500}},
			want: []packets.Payload{
				&packets.TileSet64{
					TileIndex: 0, Length: 1, Rect: packets.TileBufferRect{Width: 2}, Duration: 1,
					Colors: [64]packets.LightHsbk{{Kelvin: 3500}, {}},
				},
				&packets.TileSet64{
					TileIndex: 0, Length: 1, Rect: packets.TileBufferRect{Width: 2}, Duration: 1,
					Colors: [64]packets.LightHsbk{{}, {Kelvin: 3500}},
				},
				&packets.TileSet64{
					TileIndex: 0, Length: 1, Rect: packets.TileBufferRect{Width: 2}, Duration: 1,
					Colors: [64]packets.LightHsbk{{}, {}, {Kelvin: 3500}},
				},
				&packets.TileSet64{
					TileIndex: 0, Length: 1, Rect: packets.TileBufferRect{Width: 2}, Duration: 1,
					Colors: [64]packets.LightHsbk{{}, {}, {}, {Kelvin: 3500}},
				},
			},
		},
		"multiple tiles: sequential": {
			mode:   ChainModeSequential,
			matrix: New(2, 2, 2),
			colors: []packets.LightHsbk{{Kelvin: 3500}},
			want: []packets.Payload{
				&packets.TileSet64{
					TileIndex: 0, Length: 1, Rect: packets.TileBufferRect{Width: 2}, Duration: 1,
					Colors: [64]packets.LightHsbk{{Kelvin: 3500}, {}},
				},
				&packets.TileSet64{
					TileIndex: 0, Length: 1, Rect: packets.TileBufferRect{Width: 2}, Duration: 1,
					Colors: [64]packets.LightHsbk{{}, {Kelvin: 3500}},
				},
				&packets.TileSet64{
					TileIndex: 0, Length: 1, Rect: packets.TileBufferRect{Width: 2}, Duration: 1,
					Colors: [64]packets.LightHsbk{{}, {}, {Kelvin: 3500}},
				},
				&packets.TileSet64{
					TileIndex: 0, Length: 1, Rect: packets.TileBufferRect{Width: 2}, Duration: 1,
					Colors: [64]packets.LightHsbk{{}, {}, {}, {Kelvin: 3500}},
				},
				&packets.TileSet64{
					TileIndex: 1, Length: 1, Rect: packets.TileBufferRect{Width: 2}, Duration: 1,
					Colors: [64]packets.LightHsbk{{Kelvin: 3500}, {}},
				},
				&packets.TileSet64{
					TileIndex: 1, Length: 1, Rect: packets.TileBufferRect{Width: 2}, Duration: 1,
					Colors: [64]packets.LightHsbk{{}, {Kelvin: 3500}},
				},
				&packets.TileSet64{
					TileIndex: 1, Length: 1, Rect: packets.TileBufferRect{Width: 2}, Duration: 1,
					Colors: [64]packets.LightHsbk{{}, {}, {Kelvin: 3500}},
				},
				&packets.TileSet64{
					TileIndex: 1, Length: 1, Rect: packets.TileBufferRect{Width: 2}, Duration: 1,
					Colors: [64]packets.LightHsbk{{}, {}, {}, {Kelvin: 3500}},
				},
			},
		},
		"multiple tiles: synced": {
			mode:   ChainModeSynced,
			matrix: New(2, 2, 2),
			colors: []packets.LightHsbk{{Kelvin: 3500}},
			want: []packets.Payload{
				&packets.TileSet64{
					TileIndex: 0, Length: 2, Rect: packets.TileBufferRect{Width: 2}, Duration: 1,
					Colors: [64]packets.LightHsbk{{Kelvin: 3500}, {}},
				},
				&packets.TileSet64{
					TileIndex: 0, Length: 2, Rect: packets.TileBufferRect{Width: 2}, Duration: 1,
					Colors: [64]packets.LightHsbk{{}, {Kelvin: 3500}},
				},
				&packets.TileSet64{
					TileIndex: 0, Length: 2, Rect: packets.TileBufferRect{Width: 2}, Duration: 1,
					Colors: [64]packets.LightHsbk{{}, {}, {Kelvin: 3500}},
				},
				&packets.TileSet64{
					TileIndex: 0, Length: 2, Rect: packets.TileBufferRect{Width: 2}, Duration: 1,
					Colors: [64]packets.LightHsbk{{}, {}, {}, {Kelvin: 3500}},
				},
			},
		},
		"matrix greater than 64": {
			matrix: New(16, 8, 1),
			colors: []packets.LightHsbk{{Kelvin: 3500}},
			testSubset: func(t *testing.T, got, want []packets.Payload) {
				if len(got) != (128 * 3) {
					t.Fatal("Unexpected number of messages")
				}
				gotSubset := []packets.Payload{
					got[0], got[1], got[2],
					got[48], got[49], got[50],
					got[192], got[193], got[194],
					got[381], got[382], got[383],
				}
				assert.Equal(t, gotSubset, want)
			},
			want: []packets.Payload{
				&packets.TileSet64{
					TileIndex: 0, Length: 1, Rect: packets.TileBufferRect{Width: 16, FbIndex: 1, Y: 0},
					Colors: [64]packets.LightHsbk{
						{Kelvin: 3500}, {}, {}, {}, {}, {}, {}, {}, {}, {}, {}, {}, {}, {}, {}, {},
					},
				},
				&packets.TileSet64{
					TileIndex: 0, Length: 1, Rect: packets.TileBufferRect{Width: 16, FbIndex: 1, Y: 4},
					Colors: [64]packets.LightHsbk{},
				},
				&packets.TileCopyFrameBuffer{
					DstFbIndex: 0, SrcFbIndex: 1, Width: uint8(16),
					Height: uint8(8), Length: 1, Duration: 1,
				},
				&packets.TileSet64{
					TileIndex: 0, Length: 1, Rect: packets.TileBufferRect{Width: 16, FbIndex: 1, Y: 0},
					Colors: [64]packets.LightHsbk{
						{}, {}, {}, {}, {}, {}, {}, {}, {}, {}, {}, {}, {}, {}, {}, {},
						{Kelvin: 3500}, {}, {}, {}, {}, {}, {}, {}, {}, {}, {}, {}, {}, {}, {}, {},
					},
				},
				&packets.TileSet64{
					TileIndex: 0, Length: 1, Rect: packets.TileBufferRect{Width: 16, FbIndex: 1, Y: 4},
					Colors: [64]packets.LightHsbk{},
				},
				&packets.TileCopyFrameBuffer{
					DstFbIndex: 0, SrcFbIndex: 1, Width: uint8(16),
					Height: uint8(8), Length: 1, Duration: 1,
				},
				&packets.TileSet64{
					TileIndex: 0, Length: 1, Rect: packets.TileBufferRect{Width: 16, FbIndex: 1, Y: 0},
					Colors: [64]packets.LightHsbk{},
				},
				&packets.TileSet64{
					TileIndex: 0, Length: 1, Rect: packets.TileBufferRect{Width: 16, FbIndex: 1, Y: 4},
					Colors: [64]packets.LightHsbk{
						{Kelvin: 3500}, {}, {}, {}, {}, {}, {}, {}, {}, {}, {}, {}, {}, {}, {}, {},
					},
				},
				&packets.TileCopyFrameBuffer{
					DstFbIndex: 0, SrcFbIndex: 1, Width: uint8(16),
					Height: uint8(8), Length: 1, Duration: 1,
				},
				&packets.TileSet64{
					TileIndex: 0, Length: 1, Rect: packets.TileBufferRect{Width: 16, FbIndex: 1, Y: 0},
					Colors: [64]packets.LightHsbk{},
				},
				&packets.TileSet64{
					TileIndex: 0, Length: 1, Rect: packets.TileBufferRect{Width: 16, FbIndex: 1, Y: 4},
					Colors: [64]packets.LightHsbk{
						{}, {}, {}, {}, {}, {}, {}, {}, {}, {}, {}, {}, {}, {}, {}, {},
						{}, {}, {}, {}, {}, {}, {}, {}, {}, {}, {}, {}, {}, {}, {}, {},
						{}, {}, {}, {}, {}, {}, {}, {}, {}, {}, {}, {}, {}, {}, {}, {},
						{}, {}, {}, {}, {}, {}, {}, {}, {}, {}, {}, {}, {}, {}, {}, {Kelvin: 3500},
					},
				},
				&packets.TileCopyFrameBuffer{
					DstFbIndex: 0, SrcFbIndex: 1, Width: uint8(16),
					Height: uint8(8), Length: 1, Duration: 1,
				},
			},
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			var got []packets.Payload
			send := func(msg *protocol.Message) error {
				got = append(got, msg.Payload)
				return nil
			}
			if err := Rockets(tc.matrix, send, 1, 1, tc.mode, tc.colors...); err != tc.wantErr {
				t.Fatalf("Got error %v, want %v", err, tc.wantErr)
			}
			if tc.testSubset != nil {
				tc.testSubset(t, got, tc.want)
			} else {
				assert.Equal(t, got, tc.want)
			}
		})
	}
}

func TestWorm(t *testing.T) {
	testCases := map[string]struct {
		mode       ChainMode
		matrix     *Matrix
		size       int
		color      packets.LightHsbk
		testSubset func(t *testing.T, got, want []packets.Payload)
		want       []packets.Payload
		wantErr    error
	}{
		"single tile": {
			color:  packets.LightHsbk{Kelvin: 3500},
			matrix: New(2, 2, 2),
			size:   2,
			want: []packets.Payload{
				&packets.TileSet64{
					TileIndex: 0, Length: 1, Rect: packets.TileBufferRect{Width: 2}, Duration: 1,
					Colors: [64]packets.LightHsbk{
						{Kelvin: 3500}, {},
						{}, {},
					},
				},
				&packets.TileSet64{
					TileIndex: 0, Length: 1, Rect: packets.TileBufferRect{Width: 2}, Duration: 1,
					Colors: [64]packets.LightHsbk{
						{Kelvin: 3500}, {Kelvin: 3500},
						{}, {},
					},
				},
				&packets.TileSet64{
					TileIndex: 0, Length: 1, Rect: packets.TileBufferRect{Width: 2}, Duration: 1,
					Colors: [64]packets.LightHsbk{
						{}, {},
						{}, {Kelvin: 3500},
					},
				},
				&packets.TileSet64{
					TileIndex: 0, Length: 1, Rect: packets.TileBufferRect{Width: 2}, Duration: 1,
					Colors: [64]packets.LightHsbk{
						{}, {},
						{Kelvin: 3500}, {Kelvin: 3500},
					},
				},
				&packets.TileSet64{
					TileIndex: 0, Length: 1, Rect: packets.TileBufferRect{Width: 2}, Duration: 1,
					Colors: [64]packets.LightHsbk{
						{}, {},
						{Kelvin: 3500}, {},
					},
				},
				&packets.TileSet64{
					TileIndex: 0, Length: 1, Rect: packets.TileBufferRect{Width: 2}, Duration: 1,
					Colors: [64]packets.LightHsbk{
						{}, {},
						{}, {},
					},
				},
			},
		},
		"multiple tiles: sequential": {
			mode:   ChainModeSequential,
			matrix: New(2, 2, 2),
			color:  packets.LightHsbk{Kelvin: 3500},
			size:   2,
			want: []packets.Payload{
				&packets.TileSet64{
					TileIndex: 0, Length: 1, Rect: packets.TileBufferRect{Width: 2}, Duration: 1,
					Colors: [64]packets.LightHsbk{
						{Kelvin: 3500}, {},
						{}, {},
					},
				},
				&packets.TileSet64{
					TileIndex: 0, Length: 1, Rect: packets.TileBufferRect{Width: 2}, Duration: 1,
					Colors: [64]packets.LightHsbk{
						{Kelvin: 3500}, {Kelvin: 3500},
						{}, {},
					},
				},
				&packets.TileSet64{
					TileIndex: 0, Length: 1, Rect: packets.TileBufferRect{Width: 2}, Duration: 1,
					Colors: [64]packets.LightHsbk{
						{}, {},
						{}, {Kelvin: 3500},
					},
				},
				&packets.TileSet64{
					TileIndex: 0, Length: 1, Rect: packets.TileBufferRect{Width: 2}, Duration: 1,
					Colors: [64]packets.LightHsbk{
						{}, {},
						{Kelvin: 3500}, {Kelvin: 3500},
					},
				},
				&packets.TileSet64{
					TileIndex: 0, Length: 1, Rect: packets.TileBufferRect{Width: 2}, Duration: 1,
					Colors: [64]packets.LightHsbk{
						{}, {},
						{Kelvin: 3500}, {},
					},
				},
				&packets.TileSet64{
					TileIndex: 0, Length: 1, Rect: packets.TileBufferRect{Width: 2}, Duration: 1,
					Colors: [64]packets.LightHsbk{
						{}, {},
						{}, {},
					},
				},
				&packets.TileSet64{
					TileIndex: 1, Length: 1, Rect: packets.TileBufferRect{Width: 2}, Duration: 1,
					Colors: [64]packets.LightHsbk{
						{Kelvin: 3500}, {},
						{}, {},
					},
				},
				&packets.TileSet64{
					TileIndex: 1, Length: 1, Rect: packets.TileBufferRect{Width: 2}, Duration: 1,
					Colors: [64]packets.LightHsbk{
						{Kelvin: 3500}, {Kelvin: 3500},
						{}, {},
					},
				},
				&packets.TileSet64{
					TileIndex: 1, Length: 1, Rect: packets.TileBufferRect{Width: 2}, Duration: 1,
					Colors: [64]packets.LightHsbk{
						{}, {},
						{}, {Kelvin: 3500},
					},
				},
				&packets.TileSet64{
					TileIndex: 1, Length: 1, Rect: packets.TileBufferRect{Width: 2}, Duration: 1,
					Colors: [64]packets.LightHsbk{
						{}, {},
						{Kelvin: 3500}, {Kelvin: 3500},
					},
				},
				&packets.TileSet64{
					TileIndex: 1, Length: 1, Rect: packets.TileBufferRect{Width: 2}, Duration: 1,
					Colors: [64]packets.LightHsbk{
						{}, {},
						{Kelvin: 3500}, {},
					},
				},
				&packets.TileSet64{
					TileIndex: 1, Length: 1, Rect: packets.TileBufferRect{Width: 2}, Duration: 1,
					Colors: [64]packets.LightHsbk{
						{}, {},
						{}, {},
					},
				},
			},
		},
		"multiple tiles: synced": {
			mode:   ChainModeSynced,
			matrix: New(2, 2, 2),
			color:  packets.LightHsbk{Kelvin: 3500},
			size:   2,
			want: []packets.Payload{
				&packets.TileSet64{
					TileIndex: 0, Length: 2, Rect: packets.TileBufferRect{Width: 2}, Duration: 1,
					Colors: [64]packets.LightHsbk{
						{Kelvin: 3500}, {},
						{}, {},
					},
				},
				&packets.TileSet64{
					TileIndex: 0, Length: 2, Rect: packets.TileBufferRect{Width: 2}, Duration: 1,
					Colors: [64]packets.LightHsbk{
						{Kelvin: 3500}, {Kelvin: 3500},
						{}, {},
					},
				},
				&packets.TileSet64{
					TileIndex: 0, Length: 2, Rect: packets.TileBufferRect{Width: 2}, Duration: 1,
					Colors: [64]packets.LightHsbk{
						{}, {},
						{}, {Kelvin: 3500},
					},
				},
				&packets.TileSet64{
					TileIndex: 0, Length: 2, Rect: packets.TileBufferRect{Width: 2}, Duration: 1,
					Colors: [64]packets.LightHsbk{
						{}, {},
						{Kelvin: 3500}, {Kelvin: 3500},
					},
				},
				&packets.TileSet64{
					TileIndex: 0, Length: 2, Rect: packets.TileBufferRect{Width: 2}, Duration: 1,
					Colors: [64]packets.LightHsbk{
						{}, {},
						{Kelvin: 3500}, {},
					},
				},
				&packets.TileSet64{
					TileIndex: 0, Length: 2, Rect: packets.TileBufferRect{Width: 2}, Duration: 1,
					Colors: [64]packets.LightHsbk{
						{}, {},
						{}, {},
					},
				},
			},
		},
		"matrix greater than 64": {
			matrix: New(16, 8, 1),
			color:  packets.LightHsbk{Kelvin: 3500},
			size:   2,
			testSubset: func(t *testing.T, got, want []packets.Payload) {
				if len(got) != (390) {
					t.Fatal("Unexpected number of messages")
				}
				gotSubset := []packets.Payload{
					got[0], got[1], got[2],
					got[3], got[4], got[5],
					got[48], got[49], got[50],
					got[51], got[52], got[53],
					got[384], got[385], got[386],
					got[387], got[388], got[389],
				}
				assert.Equal(t, gotSubset, want)
			},
			want: []packets.Payload{
				&packets.TileSet64{
					TileIndex: 0, Length: 1, Rect: packets.TileBufferRect{Width: 16, FbIndex: 1, Y: 0},
					Colors: [64]packets.LightHsbk{
						{Kelvin: 3500}, {}, {}, {}, {}, {}, {}, {}, {}, {}, {}, {}, {}, {}, {}, {},
					},
				},
				&packets.TileSet64{
					TileIndex: 0, Length: 1, Rect: packets.TileBufferRect{Width: 16, FbIndex: 1, Y: 4},
					Colors: [64]packets.LightHsbk{},
				},
				&packets.TileCopyFrameBuffer{
					DstFbIndex: 0, SrcFbIndex: 1, Width: uint8(16),
					Height: uint8(8), Length: 1, Duration: 1,
				},
				&packets.TileSet64{
					TileIndex: 0, Length: 1, Rect: packets.TileBufferRect{Width: 16, FbIndex: 1, Y: 0},
					Colors: [64]packets.LightHsbk{
						{Kelvin: 3500}, {Kelvin: 3500}, {}, {}, {}, {}, {}, {}, {}, {}, {}, {}, {}, {}, {}, {},
					},
				},
				&packets.TileSet64{
					TileIndex: 0, Length: 1, Rect: packets.TileBufferRect{Width: 16, FbIndex: 1, Y: 4},
					Colors: [64]packets.LightHsbk{},
				},
				&packets.TileCopyFrameBuffer{
					DstFbIndex: 0, SrcFbIndex: 1, Width: uint8(16),
					Height: uint8(8), Length: 1, Duration: 1,
				},
				&packets.TileSet64{
					TileIndex: 0, Length: 1, Rect: packets.TileBufferRect{Width: 16, FbIndex: 1, Y: 0},
					Colors: [64]packets.LightHsbk{
						{}, {}, {}, {}, {}, {}, {}, {}, {}, {}, {}, {}, {}, {}, {}, {},
						{}, {}, {}, {}, {}, {}, {}, {}, {}, {}, {}, {}, {}, {}, {}, {Kelvin: 3500},
					},
				},
				&packets.TileSet64{
					TileIndex: 0, Length: 1, Rect: packets.TileBufferRect{Width: 16, FbIndex: 1, Y: 4},
					Colors: [64]packets.LightHsbk{},
				},
				&packets.TileCopyFrameBuffer{
					DstFbIndex: 0, SrcFbIndex: 1, Width: uint8(16),
					Height: uint8(8), Length: 1, Duration: 1,
				},
				&packets.TileSet64{
					TileIndex: 0, Length: 1, Rect: packets.TileBufferRect{Width: 16, FbIndex: 1, Y: 0},
					Colors: [64]packets.LightHsbk{
						{}, {}, {}, {}, {}, {}, {}, {}, {}, {}, {}, {}, {}, {}, {}, {},
						{}, {}, {}, {}, {}, {}, {}, {}, {}, {}, {}, {}, {}, {}, {Kelvin: 3500}, {Kelvin: 3500},
					},
				},
				&packets.TileSet64{
					TileIndex: 0, Length: 1, Rect: packets.TileBufferRect{Width: 16, FbIndex: 1, Y: 4},
					Colors: [64]packets.LightHsbk{},
				},
				&packets.TileCopyFrameBuffer{
					DstFbIndex: 0, SrcFbIndex: 1, Width: uint8(16),
					Height: uint8(8), Length: 1, Duration: 1,
				},
				&packets.TileSet64{
					TileIndex: 0, Length: 1, Rect: packets.TileBufferRect{Width: 16, FbIndex: 1, Y: 0},
					Colors: [64]packets.LightHsbk{},
				},
				&packets.TileSet64{
					TileIndex: 0, Length: 1, Rect: packets.TileBufferRect{Width: 16, FbIndex: 1, Y: 4},
					Colors: [64]packets.LightHsbk{
						{}, {}, {}, {}, {}, {}, {}, {}, {}, {}, {}, {}, {}, {}, {}, {},
						{}, {}, {}, {}, {}, {}, {}, {}, {}, {}, {}, {}, {}, {}, {}, {},
						{}, {}, {}, {}, {}, {}, {}, {}, {}, {}, {}, {}, {}, {}, {}, {},
						{Kelvin: 3500}, {}, {}, {}, {}, {}, {}, {}, {}, {}, {}, {}, {}, {}, {}, {},
					},
				},
				&packets.TileCopyFrameBuffer{
					DstFbIndex: 0, SrcFbIndex: 1, Width: uint8(16),
					Height: uint8(8), Length: 1, Duration: 1,
				},
				&packets.TileSet64{
					TileIndex: 0, Length: 1, Rect: packets.TileBufferRect{Width: 16, FbIndex: 1, Y: 0},
					Colors: [64]packets.LightHsbk{},
				},
				&packets.TileSet64{
					TileIndex: 0, Length: 1, Rect: packets.TileBufferRect{Width: 16, FbIndex: 1, Y: 4},
					Colors: [64]packets.LightHsbk{
						{}, {}, {}, {}, {}, {}, {}, {}, {}, {}, {}, {}, {}, {}, {}, {},
						{}, {}, {}, {}, {}, {}, {}, {}, {}, {}, {}, {}, {}, {}, {}, {},
						{}, {}, {}, {}, {}, {}, {}, {}, {}, {}, {}, {}, {}, {}, {}, {},
						{}, {}, {}, {}, {}, {}, {}, {}, {}, {}, {}, {}, {}, {}, {}, {},
					},
				},
				&packets.TileCopyFrameBuffer{
					DstFbIndex: 0, SrcFbIndex: 1, Width: uint8(16),
					Height: uint8(8), Length: 1, Duration: 1,
				},
			},
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			var got []packets.Payload
			send := func(msg *protocol.Message) error {
				got = append(got, msg.Payload)
				return nil
			}
			if err := Worm(tc.matrix, send, 1, 1, tc.mode, tc.size, tc.color); err != tc.wantErr {
				t.Fatalf("Got error %v, want %v", err, tc.wantErr)
			}
			if tc.testSubset != nil {
				tc.testSubset(t, got, tc.want)
			} else {
				assert.Equal(t, got, tc.want)
			}
		})
	}
}

func TestSnake(t *testing.T) {
	testCases := map[string]struct {
		mode       ChainMode
		matrix     *Matrix
		size       int
		color      packets.LightHsbk
		testSubset func(t *testing.T, got, want []packets.Payload)
		want       []packets.Payload
		wantErr    error
	}{
		"single tile": {
			matrix: New(2, 2, 2),
			color:  packets.LightHsbk{Kelvin: 3500},
			size:   2,
			want: []packets.Payload{
				&packets.TileSet64{
					TileIndex: 0, Length: 1, Rect: packets.TileBufferRect{Width: 2}, Duration: 1,
					Colors: [64]packets.LightHsbk{
						{Kelvin: 3500}, {},
						{}, {},
					},
				},
				&packets.TileSet64{
					TileIndex: 0, Length: 1, Rect: packets.TileBufferRect{Width: 2}, Duration: 1,
					Colors: [64]packets.LightHsbk{
						{Kelvin: 3500}, {Kelvin: 3500},
						{}, {},
					},
				},
				&packets.TileSet64{
					TileIndex: 0, Length: 1, Rect: packets.TileBufferRect{Width: 2}, Duration: 1,
					Colors: [64]packets.LightHsbk{
						{}, {Kelvin: 3500},
						{}, {Kelvin: 3500},
					},
				},
				&packets.TileSet64{
					TileIndex: 0, Length: 1, Rect: packets.TileBufferRect{Width: 2}, Duration: 1,
					Colors: [64]packets.LightHsbk{
						{}, {},
						{Kelvin: 3500}, {Kelvin: 3500},
					},
				},
				&packets.TileSet64{
					TileIndex: 0, Length: 1, Rect: packets.TileBufferRect{Width: 2}, Duration: 1,
					Colors: [64]packets.LightHsbk{
						{}, {},
						{Kelvin: 3500}, {},
					},
				},
				&packets.TileSet64{
					TileIndex: 0, Length: 1, Rect: packets.TileBufferRect{Width: 2}, Duration: 1,
					Colors: [64]packets.LightHsbk{
						{}, {},
						{}, {},
					},
				},
			},
		},
		"multiple tiles: sequential": {
			mode:   ChainModeSequential,
			matrix: New(2, 2, 2),
			color:  packets.LightHsbk{Kelvin: 3500},
			size:   2,
			want: []packets.Payload{
				&packets.TileSet64{
					TileIndex: 0, Length: 1, Rect: packets.TileBufferRect{Width: 2}, Duration: 1,
					Colors: [64]packets.LightHsbk{
						{Kelvin: 3500}, {},
						{}, {},
					},
				},
				&packets.TileSet64{
					TileIndex: 0, Length: 1, Rect: packets.TileBufferRect{Width: 2}, Duration: 1,
					Colors: [64]packets.LightHsbk{
						{Kelvin: 3500}, {Kelvin: 3500},
						{}, {},
					},
				},
				&packets.TileSet64{
					TileIndex: 0, Length: 1, Rect: packets.TileBufferRect{Width: 2}, Duration: 1,
					Colors: [64]packets.LightHsbk{
						{}, {Kelvin: 3500},
						{}, {Kelvin: 3500},
					},
				},
				&packets.TileSet64{
					TileIndex: 0, Length: 1, Rect: packets.TileBufferRect{Width: 2}, Duration: 1,
					Colors: [64]packets.LightHsbk{
						{}, {},
						{Kelvin: 3500}, {Kelvin: 3500},
					},
				},
				&packets.TileSet64{
					TileIndex: 0, Length: 1, Rect: packets.TileBufferRect{Width: 2}, Duration: 1,
					Colors: [64]packets.LightHsbk{
						{}, {},
						{Kelvin: 3500}, {},
					},
				},
				&packets.TileSet64{
					TileIndex: 0, Length: 1, Rect: packets.TileBufferRect{Width: 2}, Duration: 1,
					Colors: [64]packets.LightHsbk{
						{}, {},
						{}, {},
					},
				},
				&packets.TileSet64{
					TileIndex: 1, Length: 1, Rect: packets.TileBufferRect{Width: 2}, Duration: 1,
					Colors: [64]packets.LightHsbk{
						{Kelvin: 3500}, {},
						{}, {},
					},
				},
				&packets.TileSet64{
					TileIndex: 1, Length: 1, Rect: packets.TileBufferRect{Width: 2}, Duration: 1,
					Colors: [64]packets.LightHsbk{
						{Kelvin: 3500}, {Kelvin: 3500},
						{}, {},
					},
				},
				&packets.TileSet64{
					TileIndex: 1, Length: 1, Rect: packets.TileBufferRect{Width: 2}, Duration: 1,
					Colors: [64]packets.LightHsbk{
						{}, {Kelvin: 3500},
						{}, {Kelvin: 3500},
					},
				},
				&packets.TileSet64{
					TileIndex: 1, Length: 1, Rect: packets.TileBufferRect{Width: 2}, Duration: 1,
					Colors: [64]packets.LightHsbk{
						{}, {},
						{Kelvin: 3500}, {Kelvin: 3500},
					},
				},
				&packets.TileSet64{
					TileIndex: 1, Length: 1, Rect: packets.TileBufferRect{Width: 2}, Duration: 1,
					Colors: [64]packets.LightHsbk{
						{}, {},
						{Kelvin: 3500}, {},
					},
				},
				&packets.TileSet64{
					TileIndex: 1, Length: 1, Rect: packets.TileBufferRect{Width: 2}, Duration: 1,
					Colors: [64]packets.LightHsbk{
						{}, {},
						{}, {},
					},
				},
			},
		},
		"multiple tiles: synced": {
			mode:   ChainModeSynced,
			matrix: New(2, 2, 2),
			color:  packets.LightHsbk{Kelvin: 3500},
			size:   2,
			want: []packets.Payload{
				&packets.TileSet64{
					TileIndex: 0, Length: 2, Rect: packets.TileBufferRect{Width: 2}, Duration: 1,
					Colors: [64]packets.LightHsbk{
						{Kelvin: 3500}, {},
						{}, {},
					},
				},
				&packets.TileSet64{
					TileIndex: 0, Length: 2, Rect: packets.TileBufferRect{Width: 2}, Duration: 1,
					Colors: [64]packets.LightHsbk{
						{Kelvin: 3500}, {Kelvin: 3500},
						{}, {},
					},
				},
				&packets.TileSet64{
					TileIndex: 0, Length: 2, Rect: packets.TileBufferRect{Width: 2}, Duration: 1,
					Colors: [64]packets.LightHsbk{
						{}, {Kelvin: 3500},
						{}, {Kelvin: 3500},
					},
				},
				&packets.TileSet64{
					TileIndex: 0, Length: 2, Rect: packets.TileBufferRect{Width: 2}, Duration: 1,
					Colors: [64]packets.LightHsbk{
						{}, {},
						{Kelvin: 3500}, {Kelvin: 3500},
					},
				},
				&packets.TileSet64{
					TileIndex: 0, Length: 2, Rect: packets.TileBufferRect{Width: 2}, Duration: 1,
					Colors: [64]packets.LightHsbk{
						{}, {},
						{Kelvin: 3500}, {},
					},
				},
				&packets.TileSet64{
					TileIndex: 0, Length: 2, Rect: packets.TileBufferRect{Width: 2}, Duration: 1,
					Colors: [64]packets.LightHsbk{
						{}, {},
						{}, {},
					},
				},
			},
		},
		"matrix greater than 64": {
			matrix: New(16, 8, 1),
			color:  packets.LightHsbk{Kelvin: 3500},
			size:   2,
			testSubset: func(t *testing.T, got, want []packets.Payload) {
				if len(got) != (390) {
					t.Fatal("Unexpected number of messages")
				}
				gotSubset := []packets.Payload{
					got[0], got[1], got[2],
					got[3], got[4], got[5],
					got[48], got[49], got[50],
					got[51], got[52], got[53],
					got[384], got[385], got[386],
					got[387], got[388], got[389],
				}
				assert.Equal(t, gotSubset, want)
			},
			want: []packets.Payload{
				&packets.TileSet64{
					TileIndex: 0, Length: 1, Rect: packets.TileBufferRect{Width: 16, FbIndex: 1, Y: 0},
					Colors: [64]packets.LightHsbk{
						{Kelvin: 3500}, {}, {}, {}, {}, {}, {}, {}, {}, {}, {}, {}, {}, {}, {}, {},
					},
				},
				&packets.TileSet64{
					TileIndex: 0, Length: 1, Rect: packets.TileBufferRect{Width: 16, FbIndex: 1, Y: 4},
					Colors: [64]packets.LightHsbk{},
				},
				&packets.TileCopyFrameBuffer{
					DstFbIndex: 0, SrcFbIndex: 1, Width: uint8(16),
					Height: uint8(8), Length: 1, Duration: 1,
				},
				&packets.TileSet64{
					TileIndex: 0, Length: 1, Rect: packets.TileBufferRect{Width: 16, FbIndex: 1, Y: 0},
					Colors: [64]packets.LightHsbk{
						{Kelvin: 3500}, {Kelvin: 3500}, {}, {}, {}, {}, {}, {}, {}, {}, {}, {}, {}, {}, {}, {},
					},
				},
				&packets.TileSet64{
					TileIndex: 0, Length: 1, Rect: packets.TileBufferRect{Width: 16, FbIndex: 1, Y: 4},
					Colors: [64]packets.LightHsbk{},
				},
				&packets.TileCopyFrameBuffer{
					DstFbIndex: 0, SrcFbIndex: 1, Width: uint8(16),
					Height: uint8(8), Length: 1, Duration: 1,
				},
				&packets.TileSet64{
					TileIndex: 0, Length: 1, Rect: packets.TileBufferRect{Width: 16, FbIndex: 1, Y: 0},
					Colors: [64]packets.LightHsbk{
						{}, {}, {}, {}, {}, {}, {}, {}, {}, {}, {}, {}, {}, {}, {}, {Kelvin: 3500},
						{}, {}, {}, {}, {}, {}, {}, {}, {}, {}, {}, {}, {}, {}, {}, {Kelvin: 3500},
					},
				},
				&packets.TileSet64{
					TileIndex: 0, Length: 1, Rect: packets.TileBufferRect{Width: 16, FbIndex: 1, Y: 4},
					Colors: [64]packets.LightHsbk{},
				},
				&packets.TileCopyFrameBuffer{
					DstFbIndex: 0, SrcFbIndex: 1, Width: uint8(16),
					Height: uint8(8), Length: 1, Duration: 1,
				},
				&packets.TileSet64{
					TileIndex: 0, Length: 1, Rect: packets.TileBufferRect{Width: 16, FbIndex: 1, Y: 0},
					Colors: [64]packets.LightHsbk{
						{}, {}, {}, {}, {}, {}, {}, {}, {}, {}, {}, {}, {}, {}, {}, {},
						{}, {}, {}, {}, {}, {}, {}, {}, {}, {}, {}, {}, {}, {}, {Kelvin: 3500}, {Kelvin: 3500},
					},
				},
				&packets.TileSet64{
					TileIndex: 0, Length: 1, Rect: packets.TileBufferRect{Width: 16, FbIndex: 1, Y: 4},
					Colors: [64]packets.LightHsbk{},
				},
				&packets.TileCopyFrameBuffer{
					DstFbIndex: 0, SrcFbIndex: 1, Width: uint8(16),
					Height: uint8(8), Length: 1, Duration: 1,
				},
				&packets.TileSet64{
					TileIndex: 0, Length: 1, Rect: packets.TileBufferRect{Width: 16, FbIndex: 1, Y: 0},
					Colors: [64]packets.LightHsbk{},
				},
				&packets.TileSet64{
					TileIndex: 0, Length: 1, Rect: packets.TileBufferRect{Width: 16, FbIndex: 1, Y: 4},
					Colors: [64]packets.LightHsbk{
						{}, {}, {}, {}, {}, {}, {}, {}, {}, {}, {}, {}, {}, {}, {}, {},
						{}, {}, {}, {}, {}, {}, {}, {}, {}, {}, {}, {}, {}, {}, {}, {},
						{}, {}, {}, {}, {}, {}, {}, {}, {}, {}, {}, {}, {}, {}, {}, {},
						{Kelvin: 3500}, {}, {}, {}, {}, {}, {}, {}, {}, {}, {}, {}, {}, {}, {}, {},
					},
				},
				&packets.TileCopyFrameBuffer{
					DstFbIndex: 0, SrcFbIndex: 1, Width: uint8(16),
					Height: uint8(8), Length: 1, Duration: 1,
				},
				&packets.TileSet64{
					TileIndex: 0, Length: 1, Rect: packets.TileBufferRect{Width: 16, FbIndex: 1, Y: 0},
					Colors: [64]packets.LightHsbk{},
				},
				&packets.TileSet64{
					TileIndex: 0, Length: 1, Rect: packets.TileBufferRect{Width: 16, FbIndex: 1, Y: 4},
					Colors: [64]packets.LightHsbk{
						{}, {}, {}, {}, {}, {}, {}, {}, {}, {}, {}, {}, {}, {}, {}, {},
						{}, {}, {}, {}, {}, {}, {}, {}, {}, {}, {}, {}, {}, {}, {}, {},
						{}, {}, {}, {}, {}, {}, {}, {}, {}, {}, {}, {}, {}, {}, {}, {},
						{}, {}, {}, {}, {}, {}, {}, {}, {}, {}, {}, {}, {}, {}, {}, {},
					},
				},
				&packets.TileCopyFrameBuffer{
					DstFbIndex: 0, SrcFbIndex: 1, Width: uint8(16),
					Height: uint8(8), Length: 1, Duration: 1,
				},
			},
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			var got []packets.Payload
			send := func(msg *protocol.Message) error {
				got = append(got, msg.Payload)
				return nil
			}
			if err := Snake(tc.matrix, send, 1, 1, tc.mode, tc.size, tc.color); err != tc.wantErr {
				t.Fatalf("Got error %v, want %v", err, tc.wantErr)
			}
			if tc.testSubset != nil {
				tc.testSubset(t, got, tc.want)
			} else {
				assert.Equal(t, got, tc.want)
			}
		})
	}
}

func TestConcentricFrames(t *testing.T) {
	testCases := map[string]struct {
		mode      ChainMode
		matrix    *Matrix
		cycles    int
		direction AnimationDirection
		colors    []packets.LightHsbk
		want      []packets.Payload
		wantErr   error
	}{
		"single tile: inwards": {
			matrix:    New(6, 6, 2),
			cycles:    1,
			direction: AnimationDirectionInwards,
			colors:    []packets.LightHsbk{{Kelvin: 3500}},
			want: []packets.Payload{
				&packets.TileSet64{
					TileIndex: 0, Length: 1, Rect: packets.TileBufferRect{Width: 6}, Duration: 1,
					Colors: [64]packets.LightHsbk{
						{Kelvin: 3500}, {Kelvin: 3500}, {Kelvin: 3500}, {Kelvin: 3500}, {Kelvin: 3500}, {Kelvin: 3500},
						{Kelvin: 3500}, {}, {}, {}, {}, {Kelvin: 3500},
						{Kelvin: 3500}, {}, {}, {}, {}, {Kelvin: 3500},
						{Kelvin: 3500}, {}, {}, {}, {}, {Kelvin: 3500},
						{Kelvin: 3500}, {}, {}, {}, {}, {Kelvin: 3500},
						{Kelvin: 3500}, {Kelvin: 3500}, {Kelvin: 3500}, {Kelvin: 3500}, {Kelvin: 3500}, {Kelvin: 3500},
					},
				},
				&packets.TileSet64{
					TileIndex: 0, Length: 1, Rect: packets.TileBufferRect{Width: 6}, Duration: 1,
					Colors: [64]packets.LightHsbk{
						{}, {}, {}, {}, {}, {},
						{}, {Kelvin: 3500}, {Kelvin: 3500}, {Kelvin: 3500}, {Kelvin: 3500}, {},
						{}, {Kelvin: 3500}, {}, {}, {Kelvin: 3500}, {},
						{}, {Kelvin: 3500}, {}, {}, {Kelvin: 3500}, {},
						{}, {Kelvin: 3500}, {Kelvin: 3500}, {Kelvin: 3500}, {Kelvin: 3500}, {},
						{}, {}, {}, {}, {}, {},
					},
				},
				&packets.TileSet64{
					TileIndex: 0, Length: 1, Rect: packets.TileBufferRect{Width: 6}, Duration: 1,
					Colors: [64]packets.LightHsbk{
						{}, {}, {}, {}, {}, {},
						{}, {}, {}, {}, {}, {},
						{}, {}, {Kelvin: 3500}, {Kelvin: 3500}, {}, {},
						{}, {}, {Kelvin: 3500}, {Kelvin: 3500}, {}, {},
						{}, {}, {}, {}, {}, {},
						{}, {}, {}, {}, {}, {},
					},
				},
			},
		},
		"single tile: outwards": {
			matrix:    New(6, 6, 2),
			cycles:    1,
			direction: AnimationDirectionOutwards,
			colors:    []packets.LightHsbk{{Kelvin: 3500}},
			want: []packets.Payload{
				&packets.TileSet64{
					TileIndex: 0, Length: 1, Rect: packets.TileBufferRect{Width: 6}, Duration: 1,
					Colors: [64]packets.LightHsbk{
						{}, {}, {}, {}, {}, {},
						{}, {}, {}, {}, {}, {},
						{}, {}, {Kelvin: 3500}, {Kelvin: 3500}, {}, {},
						{}, {}, {Kelvin: 3500}, {Kelvin: 3500}, {}, {},
						{}, {}, {}, {}, {}, {},
						{}, {}, {}, {}, {}, {},
					},
				},
				&packets.TileSet64{
					TileIndex: 0, Length: 1, Rect: packets.TileBufferRect{Width: 6}, Duration: 1,
					Colors: [64]packets.LightHsbk{
						{}, {}, {}, {}, {}, {},
						{}, {Kelvin: 3500}, {Kelvin: 3500}, {Kelvin: 3500}, {Kelvin: 3500}, {},
						{}, {Kelvin: 3500}, {}, {}, {Kelvin: 3500}, {},
						{}, {Kelvin: 3500}, {}, {}, {Kelvin: 3500}, {},
						{}, {Kelvin: 3500}, {Kelvin: 3500}, {Kelvin: 3500}, {Kelvin: 3500}, {},
						{}, {}, {}, {}, {}, {},
					},
				},
				&packets.TileSet64{
					TileIndex: 0, Length: 1, Rect: packets.TileBufferRect{Width: 6}, Duration: 1,
					Colors: [64]packets.LightHsbk{
						{Kelvin: 3500}, {Kelvin: 3500}, {Kelvin: 3500}, {Kelvin: 3500}, {Kelvin: 3500}, {Kelvin: 3500},
						{Kelvin: 3500}, {}, {}, {}, {}, {Kelvin: 3500},
						{Kelvin: 3500}, {}, {}, {}, {}, {Kelvin: 3500},
						{Kelvin: 3500}, {}, {}, {}, {}, {Kelvin: 3500},
						{Kelvin: 3500}, {}, {}, {}, {}, {Kelvin: 3500},
						{Kelvin: 3500}, {Kelvin: 3500}, {Kelvin: 3500}, {Kelvin: 3500}, {Kelvin: 3500}, {Kelvin: 3500},
					},
				},
			},
		},
		"single tile: in-out": {
			direction: AnimationDirectionInOut,
			matrix:    New(6, 6, 2),
			cycles:    1,
			colors:    []packets.LightHsbk{{Kelvin: 3500}},
			want: []packets.Payload{
				&packets.TileSet64{
					TileIndex: 0, Length: 1, Rect: packets.TileBufferRect{Width: 6}, Duration: 1,
					Colors: [64]packets.LightHsbk{
						{Kelvin: 3500}, {Kelvin: 3500}, {Kelvin: 3500}, {Kelvin: 3500}, {Kelvin: 3500}, {Kelvin: 3500},
						{Kelvin: 3500}, {}, {}, {}, {}, {Kelvin: 3500},
						{Kelvin: 3500}, {}, {}, {}, {}, {Kelvin: 3500},
						{Kelvin: 3500}, {}, {}, {}, {}, {Kelvin: 3500},
						{Kelvin: 3500}, {}, {}, {}, {}, {Kelvin: 3500},
						{Kelvin: 3500}, {Kelvin: 3500}, {Kelvin: 3500}, {Kelvin: 3500}, {Kelvin: 3500}, {Kelvin: 3500},
					},
				},
				&packets.TileSet64{
					TileIndex: 0, Length: 1, Rect: packets.TileBufferRect{Width: 6}, Duration: 1,
					Colors: [64]packets.LightHsbk{
						{}, {}, {}, {}, {}, {},
						{}, {Kelvin: 3500}, {Kelvin: 3500}, {Kelvin: 3500}, {Kelvin: 3500}, {},
						{}, {Kelvin: 3500}, {}, {}, {Kelvin: 3500}, {},
						{}, {Kelvin: 3500}, {}, {}, {Kelvin: 3500}, {},
						{}, {Kelvin: 3500}, {Kelvin: 3500}, {Kelvin: 3500}, {Kelvin: 3500}, {},
						{}, {}, {}, {}, {}, {},
					},
				},
				&packets.TileSet64{
					TileIndex: 0, Length: 1, Rect: packets.TileBufferRect{Width: 6}, Duration: 1,
					Colors: [64]packets.LightHsbk{
						{}, {}, {}, {}, {}, {},
						{}, {}, {}, {}, {}, {},
						{}, {}, {Kelvin: 3500}, {Kelvin: 3500}, {}, {},
						{}, {}, {Kelvin: 3500}, {Kelvin: 3500}, {}, {},
						{}, {}, {}, {}, {}, {},
						{}, {}, {}, {}, {}, {},
					},
				},
				&packets.TileSet64{
					TileIndex: 0, Length: 1, Rect: packets.TileBufferRect{Width: 6}, Duration: 1,
					Colors: [64]packets.LightHsbk{
						{}, {}, {}, {}, {}, {},
						{}, {Kelvin: 3500}, {Kelvin: 3500}, {Kelvin: 3500}, {Kelvin: 3500}, {},
						{}, {Kelvin: 3500}, {}, {}, {Kelvin: 3500}, {},
						{}, {Kelvin: 3500}, {}, {}, {Kelvin: 3500}, {},
						{}, {Kelvin: 3500}, {Kelvin: 3500}, {Kelvin: 3500}, {Kelvin: 3500}, {},
						{}, {}, {}, {}, {}, {},
					},
				},
			},
		},
		"single tile: out-in": {
			direction: AnimationDirectionOutIn,
			matrix:    New(6, 6, 2),
			cycles:    1,
			colors:    []packets.LightHsbk{{Kelvin: 3500}},
			want: []packets.Payload{
				&packets.TileSet64{
					TileIndex: 0, Length: 1, Rect: packets.TileBufferRect{Width: 6}, Duration: 1,
					Colors: [64]packets.LightHsbk{
						{}, {}, {}, {}, {}, {},
						{}, {}, {}, {}, {}, {},
						{}, {}, {Kelvin: 3500}, {Kelvin: 3500}, {}, {},
						{}, {}, {Kelvin: 3500}, {Kelvin: 3500}, {}, {},
						{}, {}, {}, {}, {}, {},
						{}, {}, {}, {}, {}, {},
					},
				},
				&packets.TileSet64{
					TileIndex: 0, Length: 1, Rect: packets.TileBufferRect{Width: 6}, Duration: 1,
					Colors: [64]packets.LightHsbk{
						{}, {}, {}, {}, {}, {},
						{}, {Kelvin: 3500}, {Kelvin: 3500}, {Kelvin: 3500}, {Kelvin: 3500}, {},
						{}, {Kelvin: 3500}, {}, {}, {Kelvin: 3500}, {},
						{}, {Kelvin: 3500}, {}, {}, {Kelvin: 3500}, {},
						{}, {Kelvin: 3500}, {Kelvin: 3500}, {Kelvin: 3500}, {Kelvin: 3500}, {},
						{}, {}, {}, {}, {}, {},
					},
				},
				&packets.TileSet64{
					TileIndex: 0, Length: 1, Rect: packets.TileBufferRect{Width: 6}, Duration: 1,
					Colors: [64]packets.LightHsbk{
						{Kelvin: 3500}, {Kelvin: 3500}, {Kelvin: 3500}, {Kelvin: 3500}, {Kelvin: 3500}, {Kelvin: 3500},
						{Kelvin: 3500}, {}, {}, {}, {}, {Kelvin: 3500},
						{Kelvin: 3500}, {}, {}, {}, {}, {Kelvin: 3500},
						{Kelvin: 3500}, {}, {}, {}, {}, {Kelvin: 3500},
						{Kelvin: 3500}, {}, {}, {}, {}, {Kelvin: 3500},
						{Kelvin: 3500}, {Kelvin: 3500}, {Kelvin: 3500}, {Kelvin: 3500}, {Kelvin: 3500}, {Kelvin: 3500},
					},
				},
				&packets.TileSet64{
					TileIndex: 0, Length: 1, Rect: packets.TileBufferRect{Width: 6}, Duration: 1,
					Colors: [64]packets.LightHsbk{
						{}, {}, {}, {}, {}, {},
						{}, {Kelvin: 3500}, {Kelvin: 3500}, {Kelvin: 3500}, {Kelvin: 3500}, {},
						{}, {Kelvin: 3500}, {}, {}, {Kelvin: 3500}, {},
						{}, {Kelvin: 3500}, {}, {}, {Kelvin: 3500}, {},
						{}, {Kelvin: 3500}, {Kelvin: 3500}, {Kelvin: 3500}, {Kelvin: 3500}, {},
						{}, {}, {}, {}, {}, {},
					},
				},
			},
		},
		"multiple tiles: sequential": {
			mode:   ChainModeSequential,
			matrix: New(6, 6, 2),
			cycles: 1,
			colors: []packets.LightHsbk{{Kelvin: 3500}},
			want: []packets.Payload{
				&packets.TileSet64{
					TileIndex: 0, Length: 1, Rect: packets.TileBufferRect{Width: 6}, Duration: 1,
					Colors: [64]packets.LightHsbk{
						{Kelvin: 3500}, {Kelvin: 3500}, {Kelvin: 3500}, {Kelvin: 3500}, {Kelvin: 3500}, {Kelvin: 3500},
						{Kelvin: 3500}, {}, {}, {}, {}, {Kelvin: 3500},
						{Kelvin: 3500}, {}, {}, {}, {}, {Kelvin: 3500},
						{Kelvin: 3500}, {}, {}, {}, {}, {Kelvin: 3500},
						{Kelvin: 3500}, {}, {}, {}, {}, {Kelvin: 3500},
						{Kelvin: 3500}, {Kelvin: 3500}, {Kelvin: 3500}, {Kelvin: 3500}, {Kelvin: 3500}, {Kelvin: 3500},
					},
				},
				&packets.TileSet64{
					TileIndex: 0, Length: 1, Rect: packets.TileBufferRect{Width: 6}, Duration: 1,
					Colors: [64]packets.LightHsbk{
						{}, {}, {}, {}, {}, {},
						{}, {Kelvin: 3500}, {Kelvin: 3500}, {Kelvin: 3500}, {Kelvin: 3500}, {},
						{}, {Kelvin: 3500}, {}, {}, {Kelvin: 3500}, {},
						{}, {Kelvin: 3500}, {}, {}, {Kelvin: 3500}, {},
						{}, {Kelvin: 3500}, {Kelvin: 3500}, {Kelvin: 3500}, {Kelvin: 3500}, {},
						{}, {}, {}, {}, {}, {},
					},
				},
				&packets.TileSet64{
					TileIndex: 0, Length: 1, Rect: packets.TileBufferRect{Width: 6}, Duration: 1,
					Colors: [64]packets.LightHsbk{
						{}, {}, {}, {}, {}, {},
						{}, {}, {}, {}, {}, {},
						{}, {}, {Kelvin: 3500}, {Kelvin: 3500}, {}, {},
						{}, {}, {Kelvin: 3500}, {Kelvin: 3500}, {}, {},
						{}, {}, {}, {}, {}, {},
						{}, {}, {}, {}, {}, {},
					},
				},
				&packets.TileSet64{
					TileIndex: 1, Length: 1, Rect: packets.TileBufferRect{Width: 6}, Duration: 1,
					Colors: [64]packets.LightHsbk{
						{Kelvin: 3500}, {Kelvin: 3500}, {Kelvin: 3500}, {Kelvin: 3500}, {Kelvin: 3500}, {Kelvin: 3500},
						{Kelvin: 3500}, {}, {}, {}, {}, {Kelvin: 3500},
						{Kelvin: 3500}, {}, {}, {}, {}, {Kelvin: 3500},
						{Kelvin: 3500}, {}, {}, {}, {}, {Kelvin: 3500},
						{Kelvin: 3500}, {}, {}, {}, {}, {Kelvin: 3500},
						{Kelvin: 3500}, {Kelvin: 3500}, {Kelvin: 3500}, {Kelvin: 3500}, {Kelvin: 3500}, {Kelvin: 3500},
					},
				},
				&packets.TileSet64{
					TileIndex: 1, Length: 1, Rect: packets.TileBufferRect{Width: 6}, Duration: 1,
					Colors: [64]packets.LightHsbk{
						{}, {}, {}, {}, {}, {},
						{}, {Kelvin: 3500}, {Kelvin: 3500}, {Kelvin: 3500}, {Kelvin: 3500}, {},
						{}, {Kelvin: 3500}, {}, {}, {Kelvin: 3500}, {},
						{}, {Kelvin: 3500}, {}, {}, {Kelvin: 3500}, {},
						{}, {Kelvin: 3500}, {Kelvin: 3500}, {Kelvin: 3500}, {Kelvin: 3500}, {},
						{}, {}, {}, {}, {}, {},
					},
				},
				&packets.TileSet64{
					TileIndex: 1, Length: 1, Rect: packets.TileBufferRect{Width: 6}, Duration: 1,
					Colors: [64]packets.LightHsbk{
						{}, {}, {}, {}, {}, {},
						{}, {}, {}, {}, {}, {},
						{}, {}, {Kelvin: 3500}, {Kelvin: 3500}, {}, {},
						{}, {}, {Kelvin: 3500}, {Kelvin: 3500}, {}, {},
						{}, {}, {}, {}, {}, {},
						{}, {}, {}, {}, {}, {},
					},
				},
			},
		},
		"multiple tiles: synced": {
			mode:   ChainModeSynced,
			matrix: New(6, 6, 2),
			cycles: 1,
			colors: []packets.LightHsbk{{Kelvin: 3500}},
			want: []packets.Payload{
				&packets.TileSet64{
					TileIndex: 0, Length: 2, Rect: packets.TileBufferRect{Width: 6}, Duration: 1,
					Colors: [64]packets.LightHsbk{
						{Kelvin: 3500}, {Kelvin: 3500}, {Kelvin: 3500}, {Kelvin: 3500}, {Kelvin: 3500}, {Kelvin: 3500},
						{Kelvin: 3500}, {}, {}, {}, {}, {Kelvin: 3500},
						{Kelvin: 3500}, {}, {}, {}, {}, {Kelvin: 3500},
						{Kelvin: 3500}, {}, {}, {}, {}, {Kelvin: 3500},
						{Kelvin: 3500}, {}, {}, {}, {}, {Kelvin: 3500},
						{Kelvin: 3500}, {Kelvin: 3500}, {Kelvin: 3500}, {Kelvin: 3500}, {Kelvin: 3500}, {Kelvin: 3500},
					},
				},
				&packets.TileSet64{
					TileIndex: 0, Length: 2, Rect: packets.TileBufferRect{Width: 6}, Duration: 1,
					Colors: [64]packets.LightHsbk{
						{}, {}, {}, {}, {}, {},
						{}, {Kelvin: 3500}, {Kelvin: 3500}, {Kelvin: 3500}, {Kelvin: 3500}, {},
						{}, {Kelvin: 3500}, {}, {}, {Kelvin: 3500}, {},
						{}, {Kelvin: 3500}, {}, {}, {Kelvin: 3500}, {},
						{}, {Kelvin: 3500}, {Kelvin: 3500}, {Kelvin: 3500}, {Kelvin: 3500}, {},
						{}, {}, {}, {}, {}, {},
					},
				},
				&packets.TileSet64{
					TileIndex: 0, Length: 2, Rect: packets.TileBufferRect{Width: 6}, Duration: 1,
					Colors: [64]packets.LightHsbk{
						{}, {}, {}, {}, {}, {},
						{}, {}, {}, {}, {}, {},
						{}, {}, {Kelvin: 3500}, {Kelvin: 3500}, {}, {},
						{}, {}, {Kelvin: 3500}, {Kelvin: 3500}, {}, {},
						{}, {}, {}, {}, {}, {},
						{}, {}, {}, {}, {}, {},
					},
				},
			},
		},
		"matrix greater than 64": {
			matrix: New(16, 8, 1),
			cycles: 1,
			colors: []packets.LightHsbk{{Kelvin: 3500}},
			want: []packets.Payload{
				&packets.TileSet64{
					TileIndex: 0, Length: 1, Rect: packets.TileBufferRect{Width: 16, FbIndex: 1, Y: 0},
					Colors: [64]packets.LightHsbk{
						{Kelvin: 3500}, {Kelvin: 3500}, {Kelvin: 3500}, {Kelvin: 3500}, {Kelvin: 3500}, {Kelvin: 3500}, {Kelvin: 3500}, {Kelvin: 3500}, {Kelvin: 3500}, {Kelvin: 3500}, {Kelvin: 3500}, {Kelvin: 3500}, {Kelvin: 3500}, {Kelvin: 3500}, {Kelvin: 3500}, {Kelvin: 3500},
						{Kelvin: 3500}, {}, {}, {}, {}, {}, {}, {}, {}, {}, {}, {}, {}, {}, {}, {Kelvin: 3500},
						{Kelvin: 3500}, {}, {}, {}, {}, {}, {}, {}, {}, {}, {}, {}, {}, {}, {}, {Kelvin: 3500},
						{Kelvin: 3500}, {}, {}, {}, {}, {}, {}, {}, {}, {}, {}, {}, {}, {}, {}, {Kelvin: 3500},
					},
				},
				&packets.TileSet64{
					TileIndex: 0, Length: 1, Rect: packets.TileBufferRect{Width: 16, FbIndex: 1, Y: 4},
					Colors: [64]packets.LightHsbk{
						{Kelvin: 3500}, {}, {}, {}, {}, {}, {}, {}, {}, {}, {}, {}, {}, {}, {}, {Kelvin: 3500},
						{Kelvin: 3500}, {}, {}, {}, {}, {}, {}, {}, {}, {}, {}, {}, {}, {}, {}, {Kelvin: 3500},
						{Kelvin: 3500}, {}, {}, {}, {}, {}, {}, {}, {}, {}, {}, {}, {}, {}, {}, {Kelvin: 3500},
						{Kelvin: 3500}, {Kelvin: 3500}, {Kelvin: 3500}, {Kelvin: 3500}, {Kelvin: 3500}, {Kelvin: 3500}, {Kelvin: 3500}, {Kelvin: 3500}, {Kelvin: 3500}, {Kelvin: 3500}, {Kelvin: 3500}, {Kelvin: 3500}, {Kelvin: 3500}, {Kelvin: 3500}, {Kelvin: 3500}, {Kelvin: 3500},
					},
				},
				&packets.TileCopyFrameBuffer{
					DstFbIndex: 0, SrcFbIndex: 1, Width: uint8(16),
					Height: uint8(8), Length: 1, Duration: 1,
				},
				&packets.TileSet64{
					TileIndex: 0, Length: 1, Rect: packets.TileBufferRect{Width: 16, FbIndex: 1, Y: 0},
					Colors: [64]packets.LightHsbk{
						{}, {}, {}, {}, {}, {}, {}, {}, {}, {}, {}, {}, {}, {}, {}, {},
						{}, {Kelvin: 3500}, {Kelvin: 3500}, {Kelvin: 3500}, {Kelvin: 3500}, {Kelvin: 3500}, {Kelvin: 3500}, {Kelvin: 3500}, {Kelvin: 3500}, {Kelvin: 3500}, {Kelvin: 3500}, {Kelvin: 3500}, {Kelvin: 3500}, {Kelvin: 3500}, {Kelvin: 3500}, {},
						{}, {Kelvin: 3500}, {}, {}, {}, {}, {}, {}, {}, {}, {}, {}, {}, {}, {Kelvin: 3500}, {},
						{}, {Kelvin: 3500}, {}, {}, {}, {}, {}, {}, {}, {}, {}, {}, {}, {}, {Kelvin: 3500}, {},
					},
				},
				&packets.TileSet64{
					TileIndex: 0, Length: 1, Rect: packets.TileBufferRect{Width: 16, FbIndex: 1, Y: 4},
					Colors: [64]packets.LightHsbk{
						{}, {Kelvin: 3500}, {}, {}, {}, {}, {}, {}, {}, {}, {}, {}, {}, {}, {Kelvin: 3500}, {},
						{}, {Kelvin: 3500}, {}, {}, {}, {}, {}, {}, {}, {}, {}, {}, {}, {}, {Kelvin: 3500}, {},
						{}, {Kelvin: 3500}, {Kelvin: 3500}, {Kelvin: 3500}, {Kelvin: 3500}, {Kelvin: 3500}, {Kelvin: 3500}, {Kelvin: 3500}, {Kelvin: 3500}, {Kelvin: 3500}, {Kelvin: 3500}, {Kelvin: 3500}, {Kelvin: 3500}, {Kelvin: 3500}, {Kelvin: 3500}, {},
						{}, {}, {}, {}, {}, {}, {}, {}, {}, {}, {}, {}, {}, {}, {}, {},
					},
				},
				&packets.TileCopyFrameBuffer{
					DstFbIndex: 0, SrcFbIndex: 1, Width: uint8(16),
					Height: uint8(8), Length: 1, Duration: 1,
				},
				&packets.TileSet64{
					TileIndex: 0, Length: 1, Rect: packets.TileBufferRect{Width: 16, FbIndex: 1, Y: 0},
					Colors: [64]packets.LightHsbk{
						{}, {}, {}, {}, {}, {}, {}, {}, {}, {}, {}, {}, {}, {}, {}, {},
						{}, {}, {}, {}, {}, {}, {}, {}, {}, {}, {}, {}, {}, {}, {}, {},
						{}, {}, {Kelvin: 3500}, {Kelvin: 3500}, {Kelvin: 3500}, {Kelvin: 3500}, {Kelvin: 3500}, {Kelvin: 3500}, {Kelvin: 3500}, {Kelvin: 3500}, {Kelvin: 3500}, {Kelvin: 3500}, {Kelvin: 3500}, {Kelvin: 3500}, {}, {},
						{}, {}, {Kelvin: 3500}, {}, {}, {}, {}, {}, {}, {}, {}, {}, {}, {Kelvin: 3500}, {}, {},
					},
				},
				&packets.TileSet64{
					TileIndex: 0, Length: 1, Rect: packets.TileBufferRect{Width: 16, FbIndex: 1, Y: 4},
					Colors: [64]packets.LightHsbk{
						{}, {}, {Kelvin: 3500}, {}, {}, {}, {}, {}, {}, {}, {}, {}, {}, {Kelvin: 3500}, {}, {},
						{}, {}, {Kelvin: 3500}, {Kelvin: 3500}, {Kelvin: 3500}, {Kelvin: 3500}, {Kelvin: 3500}, {Kelvin: 3500}, {Kelvin: 3500}, {Kelvin: 3500}, {Kelvin: 3500}, {Kelvin: 3500}, {Kelvin: 3500}, {Kelvin: 3500}, {}, {},
						{}, {}, {}, {}, {}, {}, {}, {}, {}, {}, {}, {}, {}, {}, {}, {},
						{}, {}, {}, {}, {}, {}, {}, {}, {}, {}, {}, {}, {}, {}, {}, {},
					},
				},
				&packets.TileCopyFrameBuffer{
					DstFbIndex: 0, SrcFbIndex: 1, Width: uint8(16),
					Height: uint8(8), Length: 1, Duration: 1,
				},
				&packets.TileSet64{
					TileIndex: 0, Length: 1, Rect: packets.TileBufferRect{Width: 16, FbIndex: 1, Y: 0},
					Colors: [64]packets.LightHsbk{
						{}, {}, {}, {}, {}, {}, {}, {}, {}, {}, {}, {}, {}, {}, {}, {},
						{}, {}, {}, {}, {}, {}, {}, {}, {}, {}, {}, {}, {}, {}, {}, {},
						{}, {}, {}, {}, {}, {}, {}, {}, {}, {}, {}, {}, {}, {}, {}, {},
						{}, {}, {}, {Kelvin: 3500}, {Kelvin: 3500}, {Kelvin: 3500}, {Kelvin: 3500}, {Kelvin: 3500}, {Kelvin: 3500}, {Kelvin: 3500}, {Kelvin: 3500}, {Kelvin: 3500}, {Kelvin: 3500}, {}, {}, {},
					},
				},
				&packets.TileSet64{
					TileIndex: 0, Length: 1, Rect: packets.TileBufferRect{Width: 16, FbIndex: 1, Y: 4},
					Colors: [64]packets.LightHsbk{
						{}, {}, {}, {Kelvin: 3500}, {Kelvin: 3500}, {Kelvin: 3500}, {Kelvin: 3500}, {Kelvin: 3500}, {Kelvin: 3500}, {Kelvin: 3500}, {Kelvin: 3500}, {Kelvin: 3500}, {Kelvin: 3500}, {}, {}, {},
						{}, {}, {}, {}, {}, {}, {}, {}, {}, {}, {}, {}, {}, {}, {}, {},
						{}, {}, {}, {}, {}, {}, {}, {}, {}, {}, {}, {}, {}, {}, {}, {},
						{}, {}, {}, {}, {}, {}, {}, {}, {}, {}, {}, {}, {}, {}, {}, {},
					},
				},
				&packets.TileCopyFrameBuffer{
					DstFbIndex: 0, SrcFbIndex: 1, Width: uint8(16),
					Height: uint8(8), Length: 1, Duration: 1,
				},
			},
		},
		"single tile: multiple colors": {
			matrix: New(6, 6, 2),
			cycles: 2,
			colors: []packets.LightHsbk{{Kelvin: 3500}, {Kelvin: 3700}},
			want: []packets.Payload{
				&packets.TileSet64{
					TileIndex: 0, Length: 1, Rect: packets.TileBufferRect{Width: 6}, Duration: 1,
					Colors: [64]packets.LightHsbk{
						{Kelvin: 3500}, {Kelvin: 3500}, {Kelvin: 3500}, {Kelvin: 3500}, {Kelvin: 3500}, {Kelvin: 3500},
						{Kelvin: 3500}, {}, {}, {}, {}, {Kelvin: 3500},
						{Kelvin: 3500}, {}, {}, {}, {}, {Kelvin: 3500},
						{Kelvin: 3500}, {}, {}, {}, {}, {Kelvin: 3500},
						{Kelvin: 3500}, {}, {}, {}, {}, {Kelvin: 3500},
						{Kelvin: 3500}, {Kelvin: 3500}, {Kelvin: 3500}, {Kelvin: 3500}, {Kelvin: 3500}, {Kelvin: 3500},
					},
				},
				&packets.TileSet64{
					TileIndex: 0, Length: 1, Rect: packets.TileBufferRect{Width: 6}, Duration: 1,
					Colors: [64]packets.LightHsbk{
						{}, {}, {}, {}, {}, {},
						{}, {Kelvin: 3500}, {Kelvin: 3500}, {Kelvin: 3500}, {Kelvin: 3500}, {},
						{}, {Kelvin: 3500}, {}, {}, {Kelvin: 3500}, {},
						{}, {Kelvin: 3500}, {}, {}, {Kelvin: 3500}, {},
						{}, {Kelvin: 3500}, {Kelvin: 3500}, {Kelvin: 3500}, {Kelvin: 3500}, {},
						{}, {}, {}, {}, {}, {},
					},
				},
				&packets.TileSet64{
					TileIndex: 0, Length: 1, Rect: packets.TileBufferRect{Width: 6}, Duration: 1,
					Colors: [64]packets.LightHsbk{
						{}, {}, {}, {}, {}, {},
						{}, {}, {}, {}, {}, {},
						{}, {}, {Kelvin: 3500}, {Kelvin: 3500}, {}, {},
						{}, {}, {Kelvin: 3500}, {Kelvin: 3500}, {}, {},
						{}, {}, {}, {}, {}, {},
						{}, {}, {}, {}, {}, {},
					},
				},
				&packets.TileSet64{
					TileIndex: 0, Length: 1, Rect: packets.TileBufferRect{Width: 6}, Duration: 1,
					Colors: [64]packets.LightHsbk{
						{Kelvin: 3700}, {Kelvin: 3700}, {Kelvin: 3700}, {Kelvin: 3700}, {Kelvin: 3700}, {Kelvin: 3700},
						{Kelvin: 3700}, {}, {}, {}, {}, {Kelvin: 3700},
						{Kelvin: 3700}, {}, {}, {}, {}, {Kelvin: 3700},
						{Kelvin: 3700}, {}, {}, {}, {}, {Kelvin: 3700},
						{Kelvin: 3700}, {}, {}, {}, {}, {Kelvin: 3700},
						{Kelvin: 3700}, {Kelvin: 3700}, {Kelvin: 3700}, {Kelvin: 3700}, {Kelvin: 3700}, {Kelvin: 3700},
					},
				},
				&packets.TileSet64{
					TileIndex: 0, Length: 1, Rect: packets.TileBufferRect{Width: 6}, Duration: 1,
					Colors: [64]packets.LightHsbk{
						{}, {}, {}, {}, {}, {},
						{}, {Kelvin: 3700}, {Kelvin: 3700}, {Kelvin: 3700}, {Kelvin: 3700}, {},
						{}, {Kelvin: 3700}, {}, {}, {Kelvin: 3700}, {},
						{}, {Kelvin: 3700}, {}, {}, {Kelvin: 3700}, {},
						{}, {Kelvin: 3700}, {Kelvin: 3700}, {Kelvin: 3700}, {Kelvin: 3700}, {},
						{}, {}, {}, {}, {}, {},
					},
				},
				&packets.TileSet64{
					TileIndex: 0, Length: 1, Rect: packets.TileBufferRect{Width: 6}, Duration: 1,
					Colors: [64]packets.LightHsbk{
						{}, {}, {}, {}, {}, {},
						{}, {}, {}, {}, {}, {},
						{}, {}, {Kelvin: 3700}, {Kelvin: 3700}, {}, {},
						{}, {}, {Kelvin: 3700}, {Kelvin: 3700}, {}, {},
						{}, {}, {}, {}, {}, {},
						{}, {}, {}, {}, {}, {},
					},
				},
			},
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			var got []packets.Payload
			send := func(msg *protocol.Message) error {
				got = append(got, msg.Payload)
				return nil
			}
			if err := ConcentricFrames(tc.matrix, send, 1, tc.cycles, tc.mode, tc.direction, tc.colors...); err != tc.wantErr {
				t.Fatalf("Got error %v, want %v", err, tc.wantErr)
			}
			assert.Equal(t, got, tc.want)
		})
	}
}
