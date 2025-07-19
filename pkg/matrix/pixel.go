package matrix

type Pixel struct {
	X, Y int
}

// PixelCache is a convenience structure to cache pixels.
type PixelCache struct {
	pixels []*Pixel
	size   int
}

// NewPixelCache initialises a PixelCache.
func NewPixelCache(size int) *PixelCache {
	return &PixelCache{
		pixels: make([]*Pixel, size),
		size:   size,
	}
}

// GetPixel returns the Pixel pointer at the given index
// It panics if the given index is not between 0 and size.
func (c *PixelCache) GetPixel(i int) *Pixel {
	return c.pixels[i]
}

// SetPixel sets the Pixel at the given index.
// It panics if the given index is not between 0 and size.
func (c *PixelCache) SetPixel(i, x, y int) {
	if !c.IsSet(i) {
		c.pixels[i] = &Pixel{}
	}
	c.pixels[i].X = x
	c.pixels[i].Y = y
}

// IsSet returns true if a pixel at the given index is set.
func (c *PixelCache) IsSet(i int) bool {
	return c.pixels[i] != nil
}

// Pixels returns a slice of Pixel values as set in the cache.
// Any nil value is returned as a Pixel with default values.
func (c *PixelCache) Pixels() []Pixel {
	pixels := make([]Pixel, c.size)
	for i := range c.pixels {
		if c.IsSet(i) {
			pixels[i] = *c.pixels[i]
		}
	}
	return pixels
}
