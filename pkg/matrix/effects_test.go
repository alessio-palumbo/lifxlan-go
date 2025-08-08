package matrix

import (
	"testing"
	"time"

	"github.com/alessio-palumbo/lifxlan-go/pkg/protocol"
)

func TestParseChainMode(t *testing.T) {
	testCases := map[string]struct {
		value int
		want  chainMode
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
		want  animationDirection
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

// func TestWaterfall(t *testing.T) {
// 	testCases := map[string]struct {
// 		cycles int
// 		mode chainMode
// 		colors []packets.LightHsbk
// 		want  animationDirection
// 	}{
// 		"direction inwards": {
// 			value: 0, want: AnimationDirectionInwards,
// 		},
// 	}
//
// 	for name, tc := range testCases {
// 		t.Run(name, func(t *testing.T) {
// 			if got := ParseAnimationDirection(tc.value); got != tc.want {
// 				t.Fatalf("Expected %v, got %v", tc.want, got)
// 			}
// 		})
// 	}
// }
