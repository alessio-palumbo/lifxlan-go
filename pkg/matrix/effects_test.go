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
		colors  []packets.LightHsbk
		want    []*packets.TileSet64
		wantErr error
	}{
		"missing colors": {
			wantErr: ErrMissingColors,
		},
		"single tile": {
			colors: []packets.LightHsbk{{Kelvin: 3500}, {Kelvin: 3600}},
			want: []*packets.TileSet64{
				{
					TileIndex: 0, Length: 1, Rect: packets.TileBufferRect{Width: 4}, Duration: 1,
					Colors: [64]packets.LightHsbk{
						{}, {Kelvin: 3500}, {Kelvin: 3600}, {},
					},
				},
				{
					TileIndex: 0, Length: 1, Rect: packets.TileBufferRect{Width: 4}, Duration: 1,
					Colors: [64]packets.LightHsbk{
						{}, {Kelvin: 3500}, {Kelvin: 3600}, {},
						{}, {Kelvin: 3500}, {Kelvin: 3600}, {},
					},
				},
				{
					TileIndex: 0, Length: 1, Rect: packets.TileBufferRect{Width: 4}, Duration: 1,
					Colors: [64]packets.LightHsbk{
						{}, {Kelvin: 3500}, {Kelvin: 3600}, {},
						{}, {Kelvin: 3500}, {Kelvin: 3600}, {},
						{}, {Kelvin: 3500}, {Kelvin: 3600}, {},
					},
				},
				{
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
			colors: []packets.LightHsbk{{Kelvin: 3500}, {Kelvin: 3600}},
			want: []*packets.TileSet64{
				{
					TileIndex: 0, Length: 1, Rect: packets.TileBufferRect{Width: 4}, Duration: 1,
					Colors: [64]packets.LightHsbk{
						{}, {Kelvin: 3500}, {Kelvin: 3600}, {},
					},
				},
				{
					TileIndex: 0, Length: 1, Rect: packets.TileBufferRect{Width: 4}, Duration: 1,
					Colors: [64]packets.LightHsbk{
						{}, {Kelvin: 3500}, {Kelvin: 3600}, {},
						{}, {Kelvin: 3500}, {Kelvin: 3600}, {},
					},
				},
				{
					TileIndex: 0, Length: 1, Rect: packets.TileBufferRect{Width: 4}, Duration: 1,
					Colors: [64]packets.LightHsbk{
						{}, {Kelvin: 3500}, {Kelvin: 3600}, {},
						{}, {Kelvin: 3500}, {Kelvin: 3600}, {},
						{}, {Kelvin: 3500}, {Kelvin: 3600}, {},
					},
				},
				{
					TileIndex: 0, Length: 1, Rect: packets.TileBufferRect{Width: 4}, Duration: 1,
					Colors: [64]packets.LightHsbk{
						{}, {Kelvin: 3500}, {Kelvin: 3600}, {},
						{}, {Kelvin: 3500}, {Kelvin: 3600}, {},
						{}, {Kelvin: 3500}, {Kelvin: 3600}, {},
						{}, {Kelvin: 3500}, {Kelvin: 3600}, {},
					},
				},
				{
					TileIndex: 1, Length: 1, Rect: packets.TileBufferRect{Width: 4}, Duration: 1,
					Colors: [64]packets.LightHsbk{
						{}, {Kelvin: 3500}, {Kelvin: 3600}, {},
					},
				},
				{
					TileIndex: 1, Length: 1, Rect: packets.TileBufferRect{Width: 4}, Duration: 1,
					Colors: [64]packets.LightHsbk{
						{}, {Kelvin: 3500}, {Kelvin: 3600}, {},
						{}, {Kelvin: 3500}, {Kelvin: 3600}, {},
					},
				},
				{
					TileIndex: 1, Length: 1, Rect: packets.TileBufferRect{Width: 4}, Duration: 1,
					Colors: [64]packets.LightHsbk{
						{}, {Kelvin: 3500}, {Kelvin: 3600}, {},
						{}, {Kelvin: 3500}, {Kelvin: 3600}, {},
						{}, {Kelvin: 3500}, {Kelvin: 3600}, {},
					},
				},
				{
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
			colors: []packets.LightHsbk{{Kelvin: 3500}, {Kelvin: 3600}},
			want: []*packets.TileSet64{
				{
					TileIndex: 0, Length: 2, Rect: packets.TileBufferRect{Width: 4}, Duration: 1,
					Colors: [64]packets.LightHsbk{
						{}, {Kelvin: 3500}, {Kelvin: 3600}, {},
					},
				},
				{
					TileIndex: 0, Length: 2, Rect: packets.TileBufferRect{Width: 4}, Duration: 1,
					Colors: [64]packets.LightHsbk{
						{}, {Kelvin: 3500}, {Kelvin: 3600}, {},
						{}, {Kelvin: 3500}, {Kelvin: 3600}, {},
					},
				},
				{
					TileIndex: 0, Length: 2, Rect: packets.TileBufferRect{Width: 4}, Duration: 1,
					Colors: [64]packets.LightHsbk{
						{}, {Kelvin: 3500}, {Kelvin: 3600}, {},
						{}, {Kelvin: 3500}, {Kelvin: 3600}, {},
						{}, {Kelvin: 3500}, {Kelvin: 3600}, {},
					},
				},
				{
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
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			m := New(4, 4, 2)
			var got []*packets.TileSet64
			send := func(msg *protocol.Message) error {
				got = append(got, msg.Payload.(*packets.TileSet64))
				return nil
			}
			if err := Waterfall(m, send, 1, 1, tc.mode, tc.colors...); err != tc.wantErr {
				t.Fatalf("Got error %v, want %v", err, tc.wantErr)
			}
			assert.Equal(t, got, tc.want)
		})
	}
}

func TestRockets(t *testing.T) {
	testCases := map[string]struct {
		mode    ChainMode
		colors  []packets.LightHsbk
		want    []*packets.TileSet64
		wantErr error
	}{
		"missing colors": {
			wantErr: ErrMissingColors,
		},
		"single tile": {
			colors: []packets.LightHsbk{{Kelvin: 3500}},
			want: []*packets.TileSet64{
				{
					TileIndex: 0, Length: 1, Rect: packets.TileBufferRect{Width: 2}, Duration: 1,
					Colors: [64]packets.LightHsbk{{Kelvin: 3500}, {}},
				},
				{
					TileIndex: 0, Length: 1, Rect: packets.TileBufferRect{Width: 2}, Duration: 1,
					Colors: [64]packets.LightHsbk{{}, {Kelvin: 3500}},
				},
				{
					TileIndex: 0, Length: 1, Rect: packets.TileBufferRect{Width: 2}, Duration: 1,
					Colors: [64]packets.LightHsbk{{}, {}, {Kelvin: 3500}},
				},
				{
					TileIndex: 0, Length: 1, Rect: packets.TileBufferRect{Width: 2}, Duration: 1,
					Colors: [64]packets.LightHsbk{{}, {}, {}, {Kelvin: 3500}},
				},
			},
		},
		"multiple tiles: sequential": {
			mode:   ChainModeSequential,
			colors: []packets.LightHsbk{{Kelvin: 3500}},
			want: []*packets.TileSet64{
				{
					TileIndex: 0, Length: 1, Rect: packets.TileBufferRect{Width: 2}, Duration: 1,
					Colors: [64]packets.LightHsbk{{Kelvin: 3500}, {}},
				},
				{
					TileIndex: 0, Length: 1, Rect: packets.TileBufferRect{Width: 2}, Duration: 1,
					Colors: [64]packets.LightHsbk{{}, {Kelvin: 3500}},
				},
				{
					TileIndex: 0, Length: 1, Rect: packets.TileBufferRect{Width: 2}, Duration: 1,
					Colors: [64]packets.LightHsbk{{}, {}, {Kelvin: 3500}},
				},
				{
					TileIndex: 0, Length: 1, Rect: packets.TileBufferRect{Width: 2}, Duration: 1,
					Colors: [64]packets.LightHsbk{{}, {}, {}, {Kelvin: 3500}},
				},
				{
					TileIndex: 1, Length: 1, Rect: packets.TileBufferRect{Width: 2}, Duration: 1,
					Colors: [64]packets.LightHsbk{{Kelvin: 3500}, {}},
				},
				{
					TileIndex: 1, Length: 1, Rect: packets.TileBufferRect{Width: 2}, Duration: 1,
					Colors: [64]packets.LightHsbk{{}, {Kelvin: 3500}},
				},
				{
					TileIndex: 1, Length: 1, Rect: packets.TileBufferRect{Width: 2}, Duration: 1,
					Colors: [64]packets.LightHsbk{{}, {}, {Kelvin: 3500}},
				},
				{
					TileIndex: 1, Length: 1, Rect: packets.TileBufferRect{Width: 2}, Duration: 1,
					Colors: [64]packets.LightHsbk{{}, {}, {}, {Kelvin: 3500}},
				},
			},
		},
		"multiple tiles: synced": {
			mode:   ChainModeSynced,
			colors: []packets.LightHsbk{{Kelvin: 3500}},
			want: []*packets.TileSet64{
				{
					TileIndex: 0, Length: 2, Rect: packets.TileBufferRect{Width: 2}, Duration: 1,
					Colors: [64]packets.LightHsbk{{Kelvin: 3500}, {}},
				},
				{
					TileIndex: 0, Length: 2, Rect: packets.TileBufferRect{Width: 2}, Duration: 1,
					Colors: [64]packets.LightHsbk{{}, {Kelvin: 3500}},
				},
				{
					TileIndex: 0, Length: 2, Rect: packets.TileBufferRect{Width: 2}, Duration: 1,
					Colors: [64]packets.LightHsbk{{}, {}, {Kelvin: 3500}},
				},
				{
					TileIndex: 0, Length: 2, Rect: packets.TileBufferRect{Width: 2}, Duration: 1,
					Colors: [64]packets.LightHsbk{{}, {}, {}, {Kelvin: 3500}},
				},
			},
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			m := New(2, 2, 2)
			var got []*packets.TileSet64
			send := func(msg *protocol.Message) error {
				got = append(got, msg.Payload.(*packets.TileSet64))
				return nil
			}
			if err := Rockets(m, send, 1, 1, tc.mode, tc.colors...); err != tc.wantErr {
				t.Fatalf("Got error %v, want %v", err, tc.wantErr)
			}
			assert.Equal(t, got, tc.want)
		})
	}
}

func TestWorm(t *testing.T) {
	testCases := map[string]struct {
		mode    ChainMode
		size    int
		color   packets.LightHsbk
		want    []*packets.TileSet64
		wantErr error
	}{
		"single tile": {
			color: packets.LightHsbk{Kelvin: 3500},
			size:  2,
			want: []*packets.TileSet64{
				{
					TileIndex: 0, Length: 1, Rect: packets.TileBufferRect{Width: 2}, Duration: 1,
					Colors: [64]packets.LightHsbk{
						{Kelvin: 3500}, {},
						{}, {},
					},
				},
				{
					TileIndex: 0, Length: 1, Rect: packets.TileBufferRect{Width: 2}, Duration: 1,
					Colors: [64]packets.LightHsbk{
						{Kelvin: 3500}, {Kelvin: 3500},
						{}, {},
					},
				},
				{
					TileIndex: 0, Length: 1, Rect: packets.TileBufferRect{Width: 2}, Duration: 1,
					Colors: [64]packets.LightHsbk{
						{}, {},
						{}, {Kelvin: 3500},
					},
				},
				{
					TileIndex: 0, Length: 1, Rect: packets.TileBufferRect{Width: 2}, Duration: 1,
					Colors: [64]packets.LightHsbk{
						{}, {},
						{Kelvin: 3500}, {Kelvin: 3500},
					},
				},
				{
					TileIndex: 0, Length: 1, Rect: packets.TileBufferRect{Width: 2}, Duration: 1,
					Colors: [64]packets.LightHsbk{
						{}, {},
						{Kelvin: 3500}, {},
					},
				},
				{
					TileIndex: 0, Length: 1, Rect: packets.TileBufferRect{Width: 2}, Duration: 1,
					Colors: [64]packets.LightHsbk{
						{}, {},
						{}, {},
					},
				},
			},
		},
		"multiple tiles: sequential": {
			mode:  ChainModeSequential,
			color: packets.LightHsbk{Kelvin: 3500},
			size:  2,
			want: []*packets.TileSet64{
				{
					TileIndex: 0, Length: 1, Rect: packets.TileBufferRect{Width: 2}, Duration: 1,
					Colors: [64]packets.LightHsbk{
						{Kelvin: 3500}, {},
						{}, {},
					},
				},
				{
					TileIndex: 0, Length: 1, Rect: packets.TileBufferRect{Width: 2}, Duration: 1,
					Colors: [64]packets.LightHsbk{
						{Kelvin: 3500}, {Kelvin: 3500},
						{}, {},
					},
				},
				{
					TileIndex: 0, Length: 1, Rect: packets.TileBufferRect{Width: 2}, Duration: 1,
					Colors: [64]packets.LightHsbk{
						{}, {},
						{}, {Kelvin: 3500},
					},
				},
				{
					TileIndex: 0, Length: 1, Rect: packets.TileBufferRect{Width: 2}, Duration: 1,
					Colors: [64]packets.LightHsbk{
						{}, {},
						{Kelvin: 3500}, {Kelvin: 3500},
					},
				},
				{
					TileIndex: 0, Length: 1, Rect: packets.TileBufferRect{Width: 2}, Duration: 1,
					Colors: [64]packets.LightHsbk{
						{}, {},
						{Kelvin: 3500}, {},
					},
				},
				{
					TileIndex: 0, Length: 1, Rect: packets.TileBufferRect{Width: 2}, Duration: 1,
					Colors: [64]packets.LightHsbk{
						{}, {},
						{}, {},
					},
				},
				{
					TileIndex: 1, Length: 1, Rect: packets.TileBufferRect{Width: 2}, Duration: 1,
					Colors: [64]packets.LightHsbk{
						{Kelvin: 3500}, {},
						{}, {},
					},
				},
				{
					TileIndex: 1, Length: 1, Rect: packets.TileBufferRect{Width: 2}, Duration: 1,
					Colors: [64]packets.LightHsbk{
						{Kelvin: 3500}, {Kelvin: 3500},
						{}, {},
					},
				},
				{
					TileIndex: 1, Length: 1, Rect: packets.TileBufferRect{Width: 2}, Duration: 1,
					Colors: [64]packets.LightHsbk{
						{}, {},
						{}, {Kelvin: 3500},
					},
				},
				{
					TileIndex: 1, Length: 1, Rect: packets.TileBufferRect{Width: 2}, Duration: 1,
					Colors: [64]packets.LightHsbk{
						{}, {},
						{Kelvin: 3500}, {Kelvin: 3500},
					},
				},
				{
					TileIndex: 1, Length: 1, Rect: packets.TileBufferRect{Width: 2}, Duration: 1,
					Colors: [64]packets.LightHsbk{
						{}, {},
						{Kelvin: 3500}, {},
					},
				},
				{
					TileIndex: 1, Length: 1, Rect: packets.TileBufferRect{Width: 2}, Duration: 1,
					Colors: [64]packets.LightHsbk{
						{}, {},
						{}, {},
					},
				},
			},
		},
		"multiple tiles: synced": {
			mode:  ChainModeSynced,
			color: packets.LightHsbk{Kelvin: 3500},
			size:  2,
			want: []*packets.TileSet64{
				{
					TileIndex: 0, Length: 2, Rect: packets.TileBufferRect{Width: 2}, Duration: 1,
					Colors: [64]packets.LightHsbk{
						{Kelvin: 3500}, {},
						{}, {},
					},
				},
				{
					TileIndex: 0, Length: 2, Rect: packets.TileBufferRect{Width: 2}, Duration: 1,
					Colors: [64]packets.LightHsbk{
						{Kelvin: 3500}, {Kelvin: 3500},
						{}, {},
					},
				},
				{
					TileIndex: 0, Length: 2, Rect: packets.TileBufferRect{Width: 2}, Duration: 1,
					Colors: [64]packets.LightHsbk{
						{}, {},
						{}, {Kelvin: 3500},
					},
				},
				{
					TileIndex: 0, Length: 2, Rect: packets.TileBufferRect{Width: 2}, Duration: 1,
					Colors: [64]packets.LightHsbk{
						{}, {},
						{Kelvin: 3500}, {Kelvin: 3500},
					},
				},
				{
					TileIndex: 0, Length: 2, Rect: packets.TileBufferRect{Width: 2}, Duration: 1,
					Colors: [64]packets.LightHsbk{
						{}, {},
						{Kelvin: 3500}, {},
					},
				},
				{
					TileIndex: 0, Length: 2, Rect: packets.TileBufferRect{Width: 2}, Duration: 1,
					Colors: [64]packets.LightHsbk{
						{}, {},
						{}, {},
					},
				},
			},
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			m := New(2, 2, 2)
			var got []*packets.TileSet64
			send := func(msg *protocol.Message) error {
				got = append(got, msg.Payload.(*packets.TileSet64))
				return nil
			}
			if err := Worm(m, send, 1, 1, tc.mode, tc.size, tc.color); err != tc.wantErr {
				t.Fatalf("Got error %v, want %v", err, tc.wantErr)
			}
			assert.Equal(t, got, tc.want)
		})
	}
}

func TestSnake(t *testing.T) {
	testCases := map[string]struct {
		mode    ChainMode
		size    int
		color   packets.LightHsbk
		want    []*packets.TileSet64
		wantErr error
	}{
		"single tile": {
			color: packets.LightHsbk{Kelvin: 3500},
			size:  2,
			want: []*packets.TileSet64{
				{
					TileIndex: 0, Length: 1, Rect: packets.TileBufferRect{Width: 2}, Duration: 1,
					Colors: [64]packets.LightHsbk{
						{Kelvin: 3500}, {},
						{}, {},
					},
				},
				{
					TileIndex: 0, Length: 1, Rect: packets.TileBufferRect{Width: 2}, Duration: 1,
					Colors: [64]packets.LightHsbk{
						{Kelvin: 3500}, {Kelvin: 3500},
						{}, {},
					},
				},
				{
					TileIndex: 0, Length: 1, Rect: packets.TileBufferRect{Width: 2}, Duration: 1,
					Colors: [64]packets.LightHsbk{
						{}, {Kelvin: 3500},
						{}, {Kelvin: 3500},
					},
				},
				{
					TileIndex: 0, Length: 1, Rect: packets.TileBufferRect{Width: 2}, Duration: 1,
					Colors: [64]packets.LightHsbk{
						{}, {},
						{Kelvin: 3500}, {Kelvin: 3500},
					},
				},
				{
					TileIndex: 0, Length: 1, Rect: packets.TileBufferRect{Width: 2}, Duration: 1,
					Colors: [64]packets.LightHsbk{
						{}, {},
						{Kelvin: 3500}, {},
					},
				},
				{
					TileIndex: 0, Length: 1, Rect: packets.TileBufferRect{Width: 2}, Duration: 1,
					Colors: [64]packets.LightHsbk{
						{}, {},
						{}, {},
					},
				},
			},
		},
		"multiple tiles: sequential": {
			mode:  ChainModeSequential,
			color: packets.LightHsbk{Kelvin: 3500},
			size:  2,
			want: []*packets.TileSet64{
				{
					TileIndex: 0, Length: 1, Rect: packets.TileBufferRect{Width: 2}, Duration: 1,
					Colors: [64]packets.LightHsbk{
						{Kelvin: 3500}, {},
						{}, {},
					},
				},
				{
					TileIndex: 0, Length: 1, Rect: packets.TileBufferRect{Width: 2}, Duration: 1,
					Colors: [64]packets.LightHsbk{
						{Kelvin: 3500}, {Kelvin: 3500},
						{}, {},
					},
				},
				{
					TileIndex: 0, Length: 1, Rect: packets.TileBufferRect{Width: 2}, Duration: 1,
					Colors: [64]packets.LightHsbk{
						{}, {Kelvin: 3500},
						{}, {Kelvin: 3500},
					},
				},
				{
					TileIndex: 0, Length: 1, Rect: packets.TileBufferRect{Width: 2}, Duration: 1,
					Colors: [64]packets.LightHsbk{
						{}, {},
						{Kelvin: 3500}, {Kelvin: 3500},
					},
				},
				{
					TileIndex: 0, Length: 1, Rect: packets.TileBufferRect{Width: 2}, Duration: 1,
					Colors: [64]packets.LightHsbk{
						{}, {},
						{Kelvin: 3500}, {},
					},
				},
				{
					TileIndex: 0, Length: 1, Rect: packets.TileBufferRect{Width: 2}, Duration: 1,
					Colors: [64]packets.LightHsbk{
						{}, {},
						{}, {},
					},
				},
				{
					TileIndex: 1, Length: 1, Rect: packets.TileBufferRect{Width: 2}, Duration: 1,
					Colors: [64]packets.LightHsbk{
						{Kelvin: 3500}, {},
						{}, {},
					},
				},
				{
					TileIndex: 1, Length: 1, Rect: packets.TileBufferRect{Width: 2}, Duration: 1,
					Colors: [64]packets.LightHsbk{
						{Kelvin: 3500}, {Kelvin: 3500},
						{}, {},
					},
				},
				{
					TileIndex: 1, Length: 1, Rect: packets.TileBufferRect{Width: 2}, Duration: 1,
					Colors: [64]packets.LightHsbk{
						{}, {Kelvin: 3500},
						{}, {Kelvin: 3500},
					},
				},
				{
					TileIndex: 1, Length: 1, Rect: packets.TileBufferRect{Width: 2}, Duration: 1,
					Colors: [64]packets.LightHsbk{
						{}, {},
						{Kelvin: 3500}, {Kelvin: 3500},
					},
				},
				{
					TileIndex: 1, Length: 1, Rect: packets.TileBufferRect{Width: 2}, Duration: 1,
					Colors: [64]packets.LightHsbk{
						{}, {},
						{Kelvin: 3500}, {},
					},
				},
				{
					TileIndex: 1, Length: 1, Rect: packets.TileBufferRect{Width: 2}, Duration: 1,
					Colors: [64]packets.LightHsbk{
						{}, {},
						{}, {},
					},
				},
			},
		},
		"multiple tiles: synced": {
			mode:  ChainModeSynced,
			color: packets.LightHsbk{Kelvin: 3500},
			size:  2,
			want: []*packets.TileSet64{
				{
					TileIndex: 0, Length: 2, Rect: packets.TileBufferRect{Width: 2}, Duration: 1,
					Colors: [64]packets.LightHsbk{
						{Kelvin: 3500}, {},
						{}, {},
					},
				},
				{
					TileIndex: 0, Length: 2, Rect: packets.TileBufferRect{Width: 2}, Duration: 1,
					Colors: [64]packets.LightHsbk{
						{Kelvin: 3500}, {Kelvin: 3500},
						{}, {},
					},
				},
				{
					TileIndex: 0, Length: 2, Rect: packets.TileBufferRect{Width: 2}, Duration: 1,
					Colors: [64]packets.LightHsbk{
						{}, {Kelvin: 3500},
						{}, {Kelvin: 3500},
					},
				},
				{
					TileIndex: 0, Length: 2, Rect: packets.TileBufferRect{Width: 2}, Duration: 1,
					Colors: [64]packets.LightHsbk{
						{}, {},
						{Kelvin: 3500}, {Kelvin: 3500},
					},
				},
				{
					TileIndex: 0, Length: 2, Rect: packets.TileBufferRect{Width: 2}, Duration: 1,
					Colors: [64]packets.LightHsbk{
						{}, {},
						{Kelvin: 3500}, {},
					},
				},
				{
					TileIndex: 0, Length: 2, Rect: packets.TileBufferRect{Width: 2}, Duration: 1,
					Colors: [64]packets.LightHsbk{
						{}, {},
						{}, {},
					},
				},
			},
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			m := New(2, 2, 2)
			var got []*packets.TileSet64
			send := func(msg *protocol.Message) error {
				got = append(got, msg.Payload.(*packets.TileSet64))
				return nil
			}
			if err := Snake(m, send, 1, 1, tc.mode, tc.size, tc.color); err != tc.wantErr {
				t.Fatalf("Got error %v, want %v", err, tc.wantErr)
			}
			assert.Equal(t, got, tc.want)
		})
	}
}

func TestConcentricFrames(t *testing.T) {
	testCases := map[string]struct {
		mode      ChainMode
		direction AnimationDirection
		color     *packets.LightHsbk
		want      []*packets.TileSet64
		wantErr   error
	}{
		"single tile: inwards": {
			direction: AnimationDirectionInwards,
			color:     &packets.LightHsbk{Kelvin: 3500},
			want: []*packets.TileSet64{
				{
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
				{
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
				{
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
			direction: AnimationDirectionOutwards,
			color:     &packets.LightHsbk{Kelvin: 3500},
			want: []*packets.TileSet64{
				{
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
				{
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
				{
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
			color:     &packets.LightHsbk{Kelvin: 3500},
			want: []*packets.TileSet64{
				{
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
				{
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
				{
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
				{
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
			color:     &packets.LightHsbk{Kelvin: 3500},
			want: []*packets.TileSet64{
				{
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
				{
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
				{
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
				{
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
			mode:  ChainModeSequential,
			color: &packets.LightHsbk{Kelvin: 3500},
			want: []*packets.TileSet64{
				{
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
				{
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
				{
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
				{
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
				{
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
				{
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
			mode:  ChainModeSynced,
			color: &packets.LightHsbk{Kelvin: 3500},
			want: []*packets.TileSet64{
				{
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
				{
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
				{
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
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			m := New(6, 6, 2)
			var got []*packets.TileSet64
			send := func(msg *protocol.Message) error {
				got = append(got, msg.Payload.(*packets.TileSet64))
				return nil
			}
			if err := ConcentricFrames(m, send, 1, 1, tc.mode, tc.direction, tc.color); err != tc.wantErr {
				t.Fatalf("Got error %v, want %v", err, tc.wantErr)
			}
			assert.Equal(t, got, tc.want)
		})
	}
}
