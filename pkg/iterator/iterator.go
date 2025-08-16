package iterator

// IterateUp returns an iterator that yields nubers from lo to hi
func IterateUp(lo, hi int) func(yield func(int) bool) {
	return func(yield func(int) bool) {
		for i := lo; i < hi; i++ {
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
			if !yield(i) {
				return
			}
		}
	}
}

// BounceUp returns an iterator that first iterate up to n then back down.
func BounceUp(n int) func(yield func(int) bool) {
	return Chain(IterateUp(0, n), IterateDown(n-1, 1))
}

// BounceDowqn returns an iterator that first iterate down to 0 then back up.
func BounceDown(n int) func(yield func(int) bool) {
	return Chain(IterateDown(n, 0), IterateUp(1, n-1))
}

// Chain runs multiple iterators in sequence and handles early stops.
func Chain(iters ...func(yield func(int) bool)) func(yield func(int) bool) {
	return func(yield func(int) bool) {
		hasNext := true
		wrappedYield := func(v int) bool {
			if !hasNext {
				return false
			}
			hasNext = yield(v)
			return hasNext
		}
		for _, iter := range iters {
			iter(wrappedYield)
		}
	}
}
