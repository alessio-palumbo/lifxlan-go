package effects

import (
	"reflect"
	"testing"
	"time"

	"github.com/alessio-palumbo/lifxlan-go/pkg/device"
)

func TestWaterfallFrames(t *testing.T) {
	effect := NewWaterfall(WaterfallConfig{
		Capabilities: matrixCaps(4, 3),
		Colors:       []Color{color(10), color(20)},
		Cycles:       1,
	})

	got := Render(effect, time.Second, 10*time.Second)
	want := []FrameAt{
		{At: 0, Frame: testMatrixFrame(4, 3, []pixelColor{{1, 0, color(10)}, {2, 0, color(20)}})},
		{At: time.Second, Frame: testMatrixFrame(4, 3, []pixelColor{{1, 0, color(10)}, {2, 0, color(20)}, {1, 1, color(10)}, {2, 1, color(20)}})},
		{At: 2 * time.Second, Frame: testMatrixFrame(4, 3, []pixelColor{{1, 0, color(10)}, {2, 0, color(20)}, {1, 1, color(10)}, {2, 1, color(20)}, {1, 2, color(10)}, {2, 2, color(20)}})},
	}

	if !reflect.DeepEqual(got, want) {
		t.Fatalf("frames = %#v, want %#v", got, want)
	}
}

func TestRocketsFrames(t *testing.T) {
	effect := NewRockets(RocketsConfig{
		Capabilities: matrixCaps(3, 2),
		Colors:       []Color{color(10), color(20)},
		Cycles:       1,
	})

	got := Render(effect, time.Second, 10*time.Second)
	want := []FrameAt{
		{At: 0, Frame: testMatrixFrame(3, 2, []pixelColor{{0, 0, color(10)}})},
		{At: time.Second, Frame: testMatrixFrame(3, 2, []pixelColor{{1, 0, color(10)}})},
		{At: 2 * time.Second, Frame: testMatrixFrame(3, 2, []pixelColor{{2, 0, color(10)}})},
		{At: 3 * time.Second, Frame: testMatrixFrame(3, 2, []pixelColor{{0, 1, color(20)}})},
		{At: 4 * time.Second, Frame: testMatrixFrame(3, 2, []pixelColor{{1, 1, color(20)}})},
		{At: 5 * time.Second, Frame: testMatrixFrame(3, 2, []pixelColor{{2, 1, color(20)}})},
	}

	if !reflect.DeepEqual(got, want) {
		t.Fatalf("frames = %#v, want %#v", got, want)
	}
}

func TestSnakeFrames(t *testing.T) {
	effect := NewSnake(SnakeConfig{
		Capabilities: matrixCaps(3, 2),
		Size:         2,
		Color:        color(10),
		Cycles:       1,
	})

	got := Render(effect, time.Second, 10*time.Second)
	want := []FrameAt{
		{At: 0, Frame: testMatrixFrame(3, 2, []pixelColor{{0, 0, color(10)}})},
		{At: time.Second, Frame: testMatrixFrame(3, 2, []pixelColor{{0, 0, color(10)}, {1, 0, color(10)}})},
		{At: 2 * time.Second, Frame: testMatrixFrame(3, 2, []pixelColor{{1, 0, color(10)}, {2, 0, color(10)}})},
		{At: 3 * time.Second, Frame: testMatrixFrame(3, 2, []pixelColor{{2, 0, color(10)}, {2, 1, color(10)}})},
		{At: 4 * time.Second, Frame: testMatrixFrame(3, 2, []pixelColor{{2, 1, color(10)}, {1, 1, color(10)}})},
		{At: 5 * time.Second, Frame: testMatrixFrame(3, 2, []pixelColor{{1, 1, color(10)}, {0, 1, color(10)}})},
		{At: 6 * time.Second, Frame: testMatrixFrame(3, 2, []pixelColor{{0, 1, color(10)}})},
		{At: 7 * time.Second, Frame: testMatrixFrame(3, 2, nil)},
	}

	if !reflect.DeepEqual(got, want) {
		t.Fatalf("frames = %#v, want %#v", got, want)
	}
}

func TestWormFrames(t *testing.T) {
	effect := NewWorm(WormConfig{
		Capabilities: matrixCaps(3, 2),
		Size:         2,
		Color:        color(10),
		Cycles:       1,
	})

	got := Render(effect, time.Second, 10*time.Second)
	want := []FrameAt{
		{At: 0, Frame: testMatrixFrame(3, 2, []pixelColor{{0, 0, color(10)}})},
		{At: time.Second, Frame: testMatrixFrame(3, 2, []pixelColor{{0, 0, color(10)}, {1, 0, color(10)}})},
		{At: 2 * time.Second, Frame: testMatrixFrame(3, 2, []pixelColor{{2, 0, color(10)}})},
		{At: 3 * time.Second, Frame: testMatrixFrame(3, 2, []pixelColor{{2, 0, color(10)}, {2, 1, color(10)}})},
		{At: 4 * time.Second, Frame: testMatrixFrame(3, 2, []pixelColor{{1, 1, color(10)}})},
		{At: 5 * time.Second, Frame: testMatrixFrame(3, 2, []pixelColor{{1, 1, color(10)}, {0, 1, color(10)}})},
		{At: 6 * time.Second, Frame: testMatrixFrame(3, 2, []pixelColor{{0, 1, color(10)}})},
		{At: 7 * time.Second, Frame: testMatrixFrame(3, 2, nil)},
	}

	if !reflect.DeepEqual(got, want) {
		t.Fatalf("frames = %#v, want %#v", got, want)
	}
}

func TestConcentricFramesDirectionInOut(t *testing.T) {
	effect := NewConcentricFrames(ConcentricFramesConfig{
		Capabilities: matrixCaps(5, 5),
		Direction:    DirectionInOut,
		Colors:       []Color{color(10)},
		Cycles:       1,
	})

	got := Render(effect, time.Second, 10*time.Second)
	if len(got) != 4 {
		t.Fatalf("frames = %d, want 4", len(got))
	}

	wantBorders := [][]point{
		borderPoints(5, 5, 0),
		borderPoints(5, 5, 1),
		borderPoints(5, 5, 2),
		borderPoints(5, 5, 1),
	}
	for i, want := range wantBorders {
		if got := coloredPoints(got[i].Frame); !reflect.DeepEqual(got, want) {
			t.Fatalf("frame %d colored points = %#v, want %#v", i, got, want)
		}
	}
}

func TestMatrixEffectsReset(t *testing.T) {
	effects := []Effect{
		NewWaterfall(WaterfallConfig{Capabilities: matrixCaps(2, 2), Colors: []Color{color(10)}, Cycles: 1}),
		NewRockets(RocketsConfig{Capabilities: matrixCaps(2, 2), Colors: []Color{color(10)}, Cycles: 1}),
		NewSnake(SnakeConfig{Capabilities: matrixCaps(2, 2), Size: 2, Color: color(10), Cycles: 1}),
		NewWorm(WormConfig{Capabilities: matrixCaps(2, 2), Size: 2, Color: color(10), Cycles: 1}),
		NewConcentricFrames(ConcentricFramesConfig{Capabilities: matrixCaps(3, 3), Colors: []Color{color(10)}, Cycles: 1}),
	}

	for _, effect := range effects {
		first := Render(effect, time.Second, 10*time.Second)
		second := Render(effect, time.Second, 10*time.Second)
		if !reflect.DeepEqual(first, second) {
			t.Fatalf("%T second render differed after reset:\nfirst=%#v\nsecond=%#v", effect, first, second)
		}
	}
}

type pixelColor struct {
	x     int
	y     int
	color Color
}

func matrixCaps(width, height int) Capabilities {
	return Capabilities{LightType: device.LightTypeMatrix, Width: width, Height: height}
}

func testMatrixFrame(width, height int, pixels []pixelColor) Frame {
	colors := blankColors(width, height)
	for _, pixel := range pixels {
		colors[pixel.y*width+pixel.x] = pixel.color
	}
	return Frame{Colors: colors, Width: width, Height: height, Duration: time.Second}
}

func coloredPoints(frame Frame) []point {
	var points []point
	for i, color := range frame.Colors {
		if color.Brightness > 0 {
			points = append(points, point{X: i % frame.Width, Y: i / frame.Width})
		}
	}
	return points
}

func borderPoints(width, height, padding int) []point {
	var points []point
	for y := range height {
		for x := range width {
			if x < padding || x >= width-padding || y < padding || y >= height-padding {
				continue
			}
			if x == padding || x == width-1-padding || y == padding || y == height-1-padding {
				points = append(points, point{X: x, Y: y})
			}
		}
	}
	return points
}
