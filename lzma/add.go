package lzma

// add computes the sum of x and y, but panics if an overflow is
// happening.
func add(x, y int64) int64 {
	z := x + y
	if (z^x)&(z^y)&(-1<<63) != 0 {
		panic(errInt64Overflow)
	}
	return z
}
