package iterator

import "fmt"

// IterateUp returns an iterator that yields nubers from lo to hi
func IterateUp(lo, hi int) func(yield func(int) bool) {
	return func(yield func(int) bool) {
		for i := lo; i < hi; i++ {
			fmt.Println(i)
			if !yield(i) {
				return
			}
		}
	}
}

// IterateDown returns an iterator that yields nubers from n to 0.
func IterateDown(hi, lo int) func(yield func(int) bool) {
	return func(yield func(int) bool) {
		for i := hi - 1; i >= lo; i-- {
			fmt.Println(i)
			if !yield(i) {
				return
			}
		}
	}
}

// BounceUp returns an iterator that first iterate up to n then back down.
func BounceUp(n int) func(yield func(int) bool) {
	return func(yield func(int) bool) {
		IterateUp(0, n)(yield)
		IterateDown(n-1, 1)(yield)
	}
}

// BounceDowqn returns an iterator that first iterate down to 0 then back up.
func BounceDown(n int) func(yield func(int) bool) {
	return func(yield func(int) bool) {
		IterateDown(n, 0)(yield)
		IterateUp(1, n-1)(yield)
	}
}
