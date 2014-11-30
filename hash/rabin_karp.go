package hash

// A is the default constant for Robin-Karp rolling hash. This is a random
// prime.
const A = 0x97b548add41d5da1

// RabinKarp supports the computation of a rolling hash.
type RabinKarp struct {
	A uint64
	N int
	// a^n
	aOldest uint64
}

// NewRabinKarp creates a new RabinKarp value. The argument n defines the
// length of the byte sequence to be hashed. The default constant will will be
// used.
func NewRabinKarp(n int) *RabinKarp {
	return NewRabinKarpConst(n, A)
}

// NewRabinKarpConst creates a new RabinKarp value. The argument n defines the
// length of the byte sequence to be hashed. The argument a provides the
// constant used to compute the hash.
func NewRabinKarpConst(n int, a uint64) *RabinKarp {
	if n <= 0 {
		panic("number of bytes n must be positive")
	}
	aOldest := uint64(1)
	// There are faster methods. For the small n required by the LZMA
	// compressor O(n) is sufficient.
	for i := 0; i < n; i++ {
		aOldest *= a
	}
	return &RabinKarp{A: a, aOldest: aOldest, N: n}
}

// Hashes computes all hashes for the byte slice given. Note that the final
// operation for the hash computation is a multiplication by r.A. This way we
// ensure that the bits of the last byte added will spread over all bits.
func (r *RabinKarp) Hashes(p []byte) []uint64 {
	m, n := len(p), r.N
	if m < n {
		return nil
	}
	h := make([]uint64, m-n+1)
	h[0] = uint64(p[0]) * r.A
	for i := 1; i < n; i++ {
		h[0] += uint64(p[i])
		h[0] *= r.A
	}
	for i := 1; i < len(h); i++ {
		h[i] = h[i-1] - uint64(p[i-1])*r.aOldest
		h[i] += uint64(p[i+n-1])
		h[i] *= r.A
	}
	return h
}
