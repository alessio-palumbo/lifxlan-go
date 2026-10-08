package messages

import (
	"fmt"
	"math"
	"slices"
	"time"

	"github.com/alessio-palumbo/lifxlan-go/pkg/protocol"
	"github.com/alessio-palumbo/lifxprotocol-go/gen/protocol/packets"
)

const maxMatrixSequenceColors = 1_048_576

// NewMatrixFrameSequence copies and validates physical frames and returns a
// looping iterator. Each call returns fresh messages for the next complete
// frame, starting with frames[0]. Callers own timing, sending, and cancellation.
// Calls to the iterator must be serialized. No packets are preloaded or sent.
//
// All frames must have the same positive size, divisible by width, with height
// at most 255. Multi-packet frames require a width dividing 64, matching the
// row-aligned chunking of SetMatrixColorsFromSlice. The sequence is limited to
// 1,048,576 stored colors. Tile range and duration must fit their wire fields.
// These are encoding limits, not a guarantee of device support.
//
// Colors retain their exact HSBK values, including zero brightness. Coordinates
// are physical: this function does not adapt orientation, hide cells, preserve
// uplight, change power, or restore state. Frames larger than 64 colors use
// buffer 1 for staging and then copy to visible buffer 0; buffer 2 is untouched.
func NewMatrixFrameSequence(startIndex, length, width int, frames [][]packets.LightHsbk, d time.Duration) (func() []*protocol.Message, error) {
	if startIndex < 0 || startIndex > math.MaxUint8 || length <= 0 || length > math.MaxUint8 || length > 256-startIndex {
		return nil, fmt.Errorf("matrix sequence tile range must fit uint8 fields without wrapping")
	}
	if width <= 0 || width > math.MaxUint8 {
		return nil, fmt.Errorf("matrix sequence width must be 1..255")
	}
	if d < 0 || d > time.Duration(math.MaxUint32)*time.Millisecond {
		return nil, fmt.Errorf("matrix sequence duration must fit nonnegative uint32 milliseconds")
	}
	if len(frames) == 0 || len(frames) > maxMatrixSequenceColors {
		return nil, fmt.Errorf("matrix sequence requires frames within the stored-color limit")
	}
	size := len(frames[0])
	if size == 0 || size%width != 0 || size/width > math.MaxUint8 {
		return nil, fmt.Errorf("matrix sequence frames must contain complete rows with height 1..255")
	}
	if size > 64 && 64%width != 0 {
		return nil, fmt.Errorf("matrix sequence multi-packet width must divide 64 for row-aligned chunks")
	}
	if len(frames) > maxMatrixSequenceColors/size {
		return nil, fmt.Errorf("matrix sequence exceeds %d stored colors", maxMatrixSequenceColors)
	}
	for i, frame := range frames {
		if len(frame) != size {
			return nil, fmt.Errorf("matrix sequence frame %d has %d colors; expected %d", i, len(frame), size)
		}
	}
	owned := make([][]packets.LightHsbk, len(frames))
	for i, frame := range frames {
		owned[i] = slices.Clone(frame)
	}
	nextIndex := 0
	return func() []*protocol.Message {
		frame := owned[nextIndex]
		nextIndex = (nextIndex + 1) % len(owned)
		return SetMatrixColorsFromSlice(startIndex, length, width, frame, d)
	}, nil
}
