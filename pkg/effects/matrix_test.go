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

func TestSnakeDrainsTailInPathOrder(t *testing.T) {
	effect := NewSnake(SnakeConfig{
		Capabilities: matrixCaps(4, 2),
		Size:         3,
		Color:        color(10),
		Cycles:       1,
	})

	got := Render(effect, time.Second, 12*time.Second)
	want := []FrameAt{
		{At: 0, Frame: testMatrixFrame(4, 2, []pixelColor{{0, 0, color(10)}})},
		{At: time.Second, Frame: testMatrixFrame(4, 2, []pixelColor{{0, 0, color(10)}, {1, 0, color(10)}})},
		{At: 2 * time.Second, Frame: testMatrixFrame(4, 2, []pixelColor{{0, 0, color(10)}, {1, 0, color(10)}, {2, 0, color(10)}})},
		{At: 3 * time.Second, Frame: testMatrixFrame(4, 2, []pixelColor{{1, 0, color(10)}, {2, 0, color(10)}, {3, 0, color(10)}})},
		{At: 4 * time.Second, Frame: testMatrixFrame(4, 2, []pixelColor{{2, 0, color(10)}, {3, 0, color(10)}, {3, 1, color(10)}})},
		{At: 5 * time.Second, Frame: testMatrixFrame(4, 2, []pixelColor{{3, 0, color(10)}, {3, 1, color(10)}, {2, 1, color(10)}})},
		{At: 6 * time.Second, Frame: testMatrixFrame(4, 2, []pixelColor{{3, 1, color(10)}, {2, 1, color(10)}, {1, 1, color(10)}})},
		{At: 7 * time.Second, Frame: testMatrixFrame(4, 2, []pixelColor{{2, 1, color(10)}, {1, 1, color(10)}, {0, 1, color(10)}})},
		{At: 8 * time.Second, Frame: testMatrixFrame(4, 2, []pixelColor{{1, 1, color(10)}, {0, 1, color(10)}})},
		{At: 9 * time.Second, Frame: testMatrixFrame(4, 2, []pixelColor{{0, 1, color(10)}})},
		{At: 10 * time.Second, Frame: testMatrixFrame(4, 2, nil)},
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

func TestWaveFrames(t *testing.T) {
	initial := testGridMatrixFrame(4, 4)
	effect := NewWave(WaveConfig{
		Capabilities: matrixCaps(4, 4),
		Initial:      &initial,
		Cycles:       1,
	})

	got := Render(effect, time.Second, 3*time.Second)
	want := []FrameAt{
		{At: 0, Frame: testGridMatrixFrame(4, 4, []pixelColor{{0, 0, color(10)}, {0, 1, color(20)}, {0, 2, color(30)}, {0, 3, color(30)}})},
		{At: time.Second, Frame: testGridMatrixFrame(4, 4, []pixelColor{{0, 0, color(20)}, {0, 1, color(30)}, {0, 2, color(30)}, {0, 3, color(30)}, {1, 0, color(11)}, {1, 1, color(21)}, {1, 2, color(31)}, {1, 3, color(31)}})},
		{At: 2 * time.Second, Frame: testGridMatrixFrame(4, 4, []pixelColor{{0, 0, color(10)}, {0, 1, color(20)}, {0, 2, color(30)}, {0, 3, color(30)}, {1, 0, color(21)}, {1, 1, color(31)}, {1, 2, color(31)}, {1, 3, color(31)}, {2, 0, color(12)}, {2, 1, color(22)}, {2, 2, color(32)}, {2, 3, color(32)}})},
	}

	if !reflect.DeepEqual(got, want) {
		t.Fatalf("frames = %#v, want %#v", got, want)
	}
}

func TestWaveMultipleFronts(t *testing.T) {
	initial := testGridMatrixFrame(6, 4)
	effect := NewWave(WaveConfig{
		Capabilities: matrixCaps(6, 4),
		Initial:      &initial,
		Waves:        2,
		Cycles:       1,
	})

	frame, ok := effect.Next(time.Second)
	if !ok {
		t.Fatal("Next returned ok=false")
	}
	want := testGridMatrixFrame(6, 4, []pixelColor{
		{0, 0, color(10)}, {0, 1, color(20)}, {0, 2, color(30)}, {0, 3, color(30)},
		{2, 0, color(12)}, {2, 1, color(22)}, {2, 2, color(32)}, {2, 3, color(32)},
		{3, 0, color(23)}, {3, 1, color(33)}, {3, 2, color(33)}, {3, 3, color(33)},
		{4, 0, color(14)}, {4, 1, color(24)}, {4, 2, color(34)}, {4, 3, color(34)},
	})
	if !reflect.DeepEqual(frame, want) {
		t.Fatalf("frame = %#v, want %#v", frame, want)
	}
}

func TestWaveUsesInitialFrameAndClonesBase(t *testing.T) {
	base := color(200)
	special := color(220)
	initial := Frame{Colors: filledColors(4, 3, base), Width: 4, Height: 3}
	initial.Colors[2] = special
	effect := NewWave(WaveConfig{
		Capabilities: matrixCaps(4, 3),
		Initial:      &initial,
	})

	first, ok := effect.Next(time.Second)
	if !ok {
		t.Fatal("Next returned ok=false")
	}
	if first.Colors[2] != special {
		t.Fatalf("initial color was not preserved: got %#v, want %#v", first.Colors[2], special)
	}

	first.Colors[2] = color(99)
	initial.Colors[2] = color(98)
	second, ok := effect.Next(time.Second)
	if !ok {
		t.Fatal("Next returned ok=false")
	}
	if second.Colors[2] != special {
		t.Fatalf("wave shared returned or initial frame backing colors: got %#v, want %#v", second.Colors[2], special)
	}
}

func TestWaveDefaultsToSeaGradient(t *testing.T) {
	effect := NewWave(WaveConfig{
		Capabilities: matrixCaps(3, 3),
		Cycles:       1,
	})

	frame, ok := effect.Next(time.Second)
	if !ok {
		t.Fatal("Next returned ok=false")
	}
	if frame.Colors[0].Brightness == 0 || frame.Colors[6].Brightness == 0 {
		t.Fatalf("default sea gradient should not be blank: %#v", frame.Colors)
	}
	if frame.Colors[0] == frame.Colors[6] {
		t.Fatalf("default sea gradient should vary by row: %#v", frame.Colors)
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
		NewWave(WaveConfig{Capabilities: matrixCaps(2, 2), Initial: &Frame{Colors: filledColors(2, 2, color(200)), Width: 2, Height: 2}, Cycles: 1}),
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

func testGridMatrixFrame(width, height int, pixels ...[]pixelColor) Frame {
	colors := make([]Color, width*height)
	for y := range height {
		for x := range width {
			colors[y*width+x] = color(float64(y*10 + x))
		}
	}
	if len(pixels) > 0 {
		for _, pixel := range pixels[0] {
			colors[pixel.y*width+pixel.x] = pixel.color
		}
	}
	return Frame{Colors: colors, Width: width, Height: height, Duration: time.Second}
}

func filledColors(width, height int, color Color) []Color {
	colors := make([]Color, width*height)
	for i := range colors {
		colors[i] = color
	}
	return colors
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
