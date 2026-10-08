package messages

import (
	"fmt"
	"math"
	"reflect"
	"testing"
	"time"

	"github.com/alessio-palumbo/lifxprotocol-go/gen/protocol/packets"
)

func sequenceFrame(size int, brightness uint16) []packets.LightHsbk {
	frame := make([]packets.LightHsbk, size)
	for i := range frame {
		frame[i] = packets.LightHsbk{Hue: uint16(i), Saturation: 65535, Brightness: brightness, Kelvin: 3500}
	}
	return frame
}

func TestMatrixSequenceLoopsWithoutHiddenFramePreloading(t *testing.T) {
	frames := [][]packets.LightHsbk{sequenceFrame(64, 0), sequenceFrame(64, 100), sequenceFrame(64, 200), sequenceFrame(64, 300)}
	next, err := NewMatrixFrameSequence(2, 1, 8, frames, 25*time.Millisecond)
	if err != nil {
		t.Fatal(err)
	}
	for i := range 10 {
		batch := next()
		want := SetMatrixColorsFromSlice(2, 1, 8, frames[i%len(frames)], 25*time.Millisecond)
		if !reflect.DeepEqual(batch, want) {
			t.Fatalf("frame %d: got=%v want=%v", i, batch, want)
		}
		if len(batch) != 1 || batch[0].Payload.(*packets.TileSet64).Rect.FbIndex != 0 {
			t.Fatal("small frame used hidden buffer")
		}
	}
}

func TestMatrixSequenceInputAndOutputOwnership(t *testing.T) {
	frames := [][]packets.LightHsbk{sequenceFrame(64, 123)}
	next, err := NewMatrixFrameSequence(0, 1, 8, frames, time.Second)
	if err != nil {
		t.Fatal(err)
	}
	frames[0][0].Brightness = 999
	frames[0] = nil
	first := next()
	if first[0].Payload.(*packets.TileSet64).Colors[0].Brightness != 123 {
		t.Fatal("input retained")
	}
	first[0].SetSequence(42)
	first[0].SetSource(999)
	first[0].Payload.(*packets.TileSet64).Colors[0].Brightness = 999
	again := next()
	if again[0] == first[0] || again[0].Sequence() != 0 || again[0].Source() != 0 || again[0].Payload.(*packets.TileSet64).Colors[0].Brightness != 123 {
		t.Fatal("output reused or aliased")
	}
}

func TestMatrixSequenceMultiPacketStagingAndPartialPacket(t *testing.T) {
	for _, size := range []int{96, 128} {
		frames := [][]packets.LightHsbk{sequenceFrame(size, 1000)}
		next, err := NewMatrixFrameSequence(0, 1, 16, frames, 50*time.Millisecond)
		if err != nil {
			t.Fatal(err)
		}
		batch := next()
		want := SetMatrixColorsFromSlice(0, 1, 16, frames[0], 50*time.Millisecond)
		if !reflect.DeepEqual(batch, want) || len(batch) != 3 {
			t.Fatal("incorrect batch")
		}
		for _, msg := range batch[:2] {
			p := msg.Payload.(*packets.TileSet64)
			if p.Rect.FbIndex != 1 || p.Duration != 0 {
				t.Fatal("staging changed")
			}
		}
		copy := batch[2].Payload.(*packets.TileCopyFrameBuffer)
		if copy.SrcFbIndex != 1 || copy.DstFbIndex != 0 || copy.Height != uint8(size/16) || copy.Duration != 50 {
			t.Fatal(copy)
		}
		if size == 96 && batch[1].Payload.(*packets.TileSet64).Colors[32] != (packets.LightHsbk{}) {
			t.Fatal("partial packet not zero-padded")
		}
	}
}

func TestMatrixSequenceValidSmallFramesAndWireBounds(t *testing.T) {
	for _, shape := range []struct{ width, size int }{{1, 1}, {5, 55}, {7, 35}, {8, 64}} {
		next, err := NewMatrixFrameSequence(255, 1, shape.width, [][]packets.LightHsbk{sequenceFrame(shape.size, 0)}, time.Duration(math.MaxUint32)*time.Millisecond)
		if err != nil {
			t.Fatal(err)
		}
		p := next()[0].Payload.(*packets.TileSet64)
		if p.TileIndex != 255 || p.Duration != math.MaxUint32 || p.Colors[0].Brightness != 0 {
			t.Fatal("valid values changed")
		}
	}
}

func TestMatrixSequenceRejectsInvalidInputs(t *testing.T) {
	validFrames := [][]packets.LightHsbk{sequenceFrame(64, 10)}
	for _, test := range []struct {
		name                 string
		start, length, width int
		frames               [][]packets.LightHsbk
		d                    time.Duration
	}{
		{"negative start", -1, 1, 8, validFrames, 0},
		{"oversized start", 256, 1, 8, validFrames, 0},
		{"zero length", 0, 0, 8, validFrames, 0},
		{"oversized length", 0, 256, 8, validFrames, 0},
		{"wrapping range", 255, 2, 8, validFrames, 0},
		{"zero width", 0, 1, 0, validFrames, 0},
		{"oversized width", 0, 1, 256, validFrames, 0},
		{"no frames", 0, 1, 8, nil, 0},
		{"empty frame", 0, 1, 8, [][]packets.LightHsbk{nil}, 0},
		{"different sizes", 0, 1, 8, [][]packets.LightHsbk{sequenceFrame(64, 10), sequenceFrame(32, 10)}, 0},
		{"partial row", 0, 1, 8, [][]packets.LightHsbk{sequenceFrame(63, 10)}, 0},
		{"unsupported chunk width", 0, 1, 7, [][]packets.LightHsbk{sequenceFrame(70, 10)}, 0},
		{"oversized height", 0, 1, 8, [][]packets.LightHsbk{sequenceFrame(8*256, 10)}, 0},
		{"negative duration", 0, 1, 8, validFrames, -time.Second},
		{"oversized duration", 0, 1, 8, validFrames, time.Duration(math.MaxUint32)*time.Millisecond + time.Millisecond},
	} {
		t.Run(test.name, func(t *testing.T) {
			next, err := NewMatrixFrameSequence(test.start, test.length, test.width, test.frames, test.d)
			if err == nil || next != nil {
				t.Fatalf("next=%v err=%v", next == nil, err)
			}
		})
	}
	frames := make([][]packets.LightHsbk, maxMatrixSequenceColors/64+1)
	for i := range frames {
		frames[i] = validFrames[0]
	}
	if _, err := NewMatrixFrameSequence(0, 1, 8, frames, 0); err == nil {
		t.Fatal("stored color limit ignored")
	}
}

func ExampleNewMatrixFrameSequence() {
	frames := [][]packets.LightHsbk{
		{{Brightness: 65535, Kelvin: 3500}},
		{{Brightness: 32768, Kelvin: 3500}},
		{{Brightness: 0, Kelvin: 3500}},
	}
	next, err := NewMatrixFrameSequence(0, 1, 1, frames, 100*time.Millisecond)
	if err != nil {
		panic(err)
	}
	for range 4 {
		batch := next() // Send the entire batch at the caller's chosen frequency.
		fmt.Println(batch[0].Payload.(*packets.TileSet64).Colors[0].Brightness)
	}
	// Output:
	// 65535
	// 32768
	// 0
	// 65535
}
