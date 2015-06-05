// The package i64 provides basic functions supporting the int64 type.
package i64

// Minimum and maximum value for the int64 type.
const (
	Min = -1 << 63
	Max = 1<<63 - 1
)

// Add adds x and y and detects overflow.
func Add(x, y int64) (z int64, overflow bool) {
	z = x + y
	return z, (z^x)&(z^y)&Min != 0
}

// Sub computes x-y and detects overflow.
func Sub(x, y int64) (z int64, overflow bool) {
	z = x - y
	return z, (z^x) & ^(z^y) & Min != 0
}
