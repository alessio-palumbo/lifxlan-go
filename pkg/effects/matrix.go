package effects

import "time"

// Direction defines how ConcentricFrames moves between matrix borders.
type Direction int

const (
	// DirectionInwards draws borders from the outside in.
	DirectionInwards Direction = iota
	// DirectionOutwards draws borders from the inside out.
	DirectionOutwards
	// DirectionInOut draws borders from the outside in, then back out.
	DirectionInOut
	// DirectionOutIn draws borders from the inside out, then back in.
	DirectionOutIn
)

// WaterfallConfig configures a Waterfall effect.
type WaterfallConfig struct {
	Capabilities Capabilities
	Colors       []Color
	Cycles       int
}

// Waterfall fills rows cumulatively with a centered color strip.
type Waterfall struct {
	cfg    WaterfallConfig
	step   int
	colors []Color
}

// NewWaterfall returns a Waterfall effect.
func NewWaterfall(cfg WaterfallConfig) *Waterfall {
	return &Waterfall{cfg: cfg}
}

// Next returns the next waterfall frame.
func (w *Waterfall) Next(dt time.Duration) (Frame, bool) {
	width, height := frameDimensions(w.cfg.Capabilities)
	if w.done(height) {
		return Frame{}, false
	}
	if w.step%height == 0 {
		w.colors = blankColors(width, height)
	}

	colors := matrixColors(w.cfg.Colors)
	colors = colors[:min(len(colors), width)]
	x := (width - len(colors)) / 2
	y := w.step % height
	setColors(w.colors, width, x, y, colors...)

	w.step++
	return matrixFrame(w.colors, width, height, dt), true
}

// Reset resets the effect.
func (w *Waterfall) Reset() {
	w.step = 0
	w.colors = nil
}

func (w *Waterfall) done(stepsPerCycle int) bool {
	return w.cfg.Cycles > 0 && w.step >= w.cfg.Cycles*stepsPerCycle
}

// RocketsConfig configures a Rockets effect.
type RocketsConfig struct {
	Capabilities Capabilities
	Colors       []Color
	Cycles       int
}

// Rockets moves a single pixel through the matrix in row-major order.
type Rockets struct {
	cfg  RocketsConfig
	step int
}

// NewRockets returns a Rockets effect.
func NewRockets(cfg RocketsConfig) *Rockets {
	return &Rockets{cfg: cfg}
}

// Next returns the next rockets frame.
func (r *Rockets) Next(dt time.Duration) (Frame, bool) {
	width, height := frameDimensions(r.cfg.Capabilities)
	size := width * height
	if r.done(size) {
		return Frame{}, false
	}

	step := r.step % size
	x, y := step%width, step/width
	palette := matrixColors(r.cfg.Colors)
	colors := blankColors(width, height)
	setPixel(colors, width, x, y, palette[y%len(palette)])

	r.step++
	return matrixFrame(colors, width, height, dt), true
}

// Reset resets the effect.
func (r *Rockets) Reset() {
	r.step = 0
}

func (r *Rockets) done(stepsPerCycle int) bool {
	return r.cfg.Cycles > 0 && r.step >= r.cfg.Cycles*stepsPerCycle
}

// SnakeConfig configures a Snake effect.
type SnakeConfig struct {
	Capabilities Capabilities
	Size         int
	Color        Color
	Cycles       int
}

// Snake moves a trailing segment through a serpentine matrix path.
type Snake struct {
	cfg    SnakeConfig
	step   int
	colors []Color
	cache  []point
	set    []bool
}

// NewSnake returns a Snake effect.
func NewSnake(cfg SnakeConfig) *Snake {
	return &Snake{cfg: cfg}
}

// Next returns the next snake frame.
func (s *Snake) Next(dt time.Duration) (Frame, bool) {
	width, height := frameDimensions(s.cfg.Capabilities)
	size := width * height
	snakeSize := tailSize(s.cfg.Size, width)
	stepsPerCycle := size + snakeSize
	if s.done(stepsPerCycle) {
		return Frame{}, false
	}
	if s.step%stepsPerCycle == 0 {
		s.resetState(width, height, snakeSize)
	}

	pos := s.step % stepsPerCycle
	if pos < size {
		slot := pos % snakeSize
		if s.set[slot] {
			clearPixel(s.colors, width, s.cache[slot])
		}
		s.cache[slot] = serpentinePoint(width, pos)
		s.set[slot] = true
		setPixel(s.colors, width, s.cache[slot].X, s.cache[slot].Y, s.cfg.Color)
	} else {
		slot := pos % snakeSize
		if s.set[slot] {
			clearPixel(s.colors, width, s.cache[slot])
			s.set[slot] = false
		}
	}

	s.step++
	return matrixFrame(s.colors, width, height, dt), true
}

// Reset resets the effect.
func (s *Snake) Reset() {
	s.step = 0
	s.colors = nil
	s.cache = nil
	s.set = nil
}

func (s *Snake) resetState(width, height, snakeSize int) {
	s.colors = blankColors(width, height)
	s.cache = make([]point, snakeSize)
	s.set = make([]bool, snakeSize)
}

func (s *Snake) done(stepsPerCycle int) bool {
	return s.cfg.Cycles > 0 && s.step >= s.cfg.Cycles*stepsPerCycle
}

// WormConfig configures a Worm effect.
type WormConfig struct {
	Capabilities Capabilities
	Size         int
	Color        Color
	Cycles       int
}

// Worm moves short batches of pixels through a serpentine matrix path.
type Worm struct {
	cfg       WormConfig
	step      int
	colors    []Color
	cache     []point
	set       []bool
	pixelsSet int
}

// NewWorm returns a Worm effect.
func NewWorm(cfg WormConfig) *Worm {
	return &Worm{cfg: cfg}
}

// Next returns the next worm frame.
func (w *Worm) Next(dt time.Duration) (Frame, bool) {
	width, height := frameDimensions(w.cfg.Capabilities)
	size := width * height
	wormSize := tailSize(w.cfg.Size, width)
	stepsPerCycle := size + wormSize
	if w.done(stepsPerCycle) {
		return Frame{}, false
	}
	if w.step%stepsPerCycle == 0 {
		w.resetState(width, height, wormSize)
	}

	pos := w.step % stepsPerCycle
	if pos < size {
		if w.pixelsSet == wormSize {
			for i, isSet := range w.set {
				if isSet {
					clearPixel(w.colors, width, w.cache[i])
					w.set[i] = false
				}
			}
			w.pixelsSet = 0
		}
		slot := pos % wormSize
		w.cache[slot] = serpentinePoint(width, pos)
		w.set[slot] = true
		w.pixelsSet++
		setPixel(w.colors, width, w.cache[slot].X, w.cache[slot].Y, w.cfg.Color)
	} else {
		slot := pos - size
		if w.set[slot] {
			clearPixel(w.colors, width, w.cache[slot])
			w.set[slot] = false
		}
	}

	w.step++
	return matrixFrame(w.colors, width, height, dt), true
}

// Reset resets the effect.
func (w *Worm) Reset() {
	w.step = 0
	w.colors = nil
	w.cache = nil
	w.set = nil
	w.pixelsSet = 0
}

func (w *Worm) resetState(width, height, wormSize int) {
	w.colors = blankColors(width, height)
	w.cache = make([]point, wormSize)
	w.set = make([]bool, wormSize)
	w.pixelsSet = 0
}

func (w *Worm) done(stepsPerCycle int) bool {
	return w.cfg.Cycles > 0 && w.step >= w.cfg.Cycles*stepsPerCycle
}

// ConcentricFramesConfig configures a ConcentricFrames effect.
type ConcentricFramesConfig struct {
	Capabilities Capabilities
	Direction    Direction
	Colors       []Color
	Cycles       int
}

// ConcentricFrames draws matrix borders according to Direction.
type ConcentricFrames struct {
	cfg  ConcentricFramesConfig
	step int
}

// NewConcentricFrames returns a ConcentricFrames effect.
func NewConcentricFrames(cfg ConcentricFramesConfig) *ConcentricFrames {
	return &ConcentricFrames{cfg: cfg}
}

// Next returns the next concentric frame.
func (c *ConcentricFrames) Next(dt time.Duration) (Frame, bool) {
	width, height := frameDimensions(c.cfg.Capabilities)
	sequence := paddingSequence(width, height, c.cfg.Direction)
	if c.done(len(sequence)) {
		return Frame{}, false
	}

	pos := c.step % len(sequence)
	cycle := c.step / len(sequence)
	palette := matrixColors(c.cfg.Colors)
	colors := blankColors(width, height)
	setBorder(colors, width, height, sequence[pos], palette[cycle%len(palette)])

	c.step++
	return matrixFrame(colors, width, height, dt), true
}

// Reset resets the effect.
func (c *ConcentricFrames) Reset() {
	c.step = 0
}

func (c *ConcentricFrames) done(stepsPerCycle int) bool {
	return c.cfg.Cycles > 0 && c.step >= c.cfg.Cycles*stepsPerCycle
}

type point struct {
	X int
	Y int
}

func blankColors(width, height int) []Color {
	colors := make([]Color, width*height)
	for i := range colors {
		colors[i] = blankColor
	}
	return colors
}

func matrixColors(colors []Color) []Color {
	if len(colors) == 0 {
		return []Color{DefaultColor}
	}
	return colors
}

func matrixFrame(colors []Color, width, height int, duration time.Duration) Frame {
	return Frame{
		Colors:   append([]Color(nil), colors...),
		Width:    width,
		Height:   height,
		Duration: duration,
	}
}

func setColors(colors []Color, width, x, y int, values ...Color) {
	for _, color := range values {
		if x >= width {
			x = 0
			y++
		}
		if y >= len(colors)/width {
			return
		}
		setPixel(colors, width, x, y, color)
		x++
	}
}

func setPixel(colors []Color, width, x, y int, color Color) {
	colors[y*width+x] = color
}

func clearPixel(colors []Color, width int, p point) {
	setPixel(colors, width, p.X, p.Y, blankColor)
}

func serpentinePoint(width, index int) point {
	x, y := index%width, index/width
	if y%2 == 1 {
		x = width - 1 - x
	}
	return point{X: x, Y: y}
}

func tailSize(size, width int) int {
	return min(max(size, 1), width)
}

func paddingSequence(width, height int, direction Direction) []int {
	maxSteps := min((width-1)/2, (height-1)/2) + 1
	switch direction {
	case DirectionOutwards:
		return iterateDown(maxSteps, 0)
	case DirectionInOut:
		return append(iterateUp(0, maxSteps), iterateDown(maxSteps-1, 1)...)
	case DirectionOutIn:
		return append(iterateDown(maxSteps, 0), iterateUp(1, maxSteps-1)...)
	default:
		return iterateUp(0, maxSteps)
	}
}

func iterateUp(lo, hi int) []int {
	values := make([]int, 0, max(hi-lo, 0))
	for i := lo; i < hi; i++ {
		values = append(values, i)
	}
	return values
}

func iterateDown(hi, lo int) []int {
	values := make([]int, 0, max(hi-lo, 0))
	for i := hi - 1; i >= lo; i-- {
		values = append(values, i)
	}
	return values
}

func setBorder(colors []Color, width, height, padding int, color Color) {
	padding = min(padding, min((width-1)/2, (height-1)/2))
	x0, y0 := padding, padding
	x1, y1 := width-1-padding, height-1-padding

	for x := x0; x <= x1; x++ {
		setPixel(colors, width, x, y0, color)
		setPixel(colors, width, x, y1, color)
	}
	for y := y0; y <= y1; y++ {
		setPixel(colors, width, x0, y, color)
		setPixel(colors, width, x1, y, color)
	}
}
