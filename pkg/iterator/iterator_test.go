package iterator

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestIterateUp(t *testing.T) {
	testCases := map[string]struct {
		lo, hi int
		want   []int
	}{
		"no range": {
			lo: 0, hi: 0,
		},
		"inverted range": {
			lo: 4, hi: 0,
		},
		"correct range": {
			lo: 0, hi: 4, want: []int{0, 1, 2, 3},
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			var got []int
			for v := range IterateUp(tc.lo, tc.hi) {
				got = append(got, v)
			}
			assert.Equal(t, tc.want, got)
		})
	}
}

func TestIterateDown(t *testing.T) {
	testCases := map[string]struct {
		hi, lo int
		want   []int
	}{
		"no range": {
			hi: 0, lo: 0,
		},
		"inverted range": {
			hi: 0, lo: 4,
		},
		"correct range": {
			hi: 4, lo: 0, want: []int{3, 2, 1, 0},
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			var got []int
			for v := range IterateDown(tc.hi, tc.lo) {
				got = append(got, v)
			}
			assert.Equal(t, tc.want, got)
		})
	}
}

func TestBounceUp(t *testing.T) {
	testCases := map[string]struct {
		n        int
		stopFunc func(int) bool
		want     []int
	}{
		"no range": {
			n: 0,
		},
		"negative n": {
			n: -1,
		},
		"can stop midway": {
			n:        4,
			stopFunc: func(n int) bool { return n == 3 },
			want:     []int{0, 1, 2},
		},
		"positive n": {
			n: 4, want: []int{0, 1, 2, 3, 2, 1},
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			var got []int
			for v := range BounceUp(tc.n) {
				if tc.stopFunc != nil && tc.stopFunc(v) {
					break
				}
				got = append(got, v)
			}
			assert.Equal(t, tc.want, got)
		})
	}
}

func TestBounceDown(t *testing.T) {
	testCases := map[string]struct {
		n        int
		stopFunc func(int) bool
		want     []int
	}{
		"no range": {
			n: 0,
		},
		"negative n": {
			n: -1,
		},
		"can stop midway": {
			n:        4,
			stopFunc: func(n int) bool { return n == 0 },
			want:     []int{3, 2, 1},
		},
		"positive n": {
			n: 4, want: []int{3, 2, 1, 0, 1, 2},
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			var got []int
			for v := range BounceDown(tc.n) {
				if tc.stopFunc != nil && tc.stopFunc(v) {
					break
				}
				got = append(got, v)
			}
			assert.Equal(t, tc.want, got)
		})
	}
}
