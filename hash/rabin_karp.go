package hash

// A is the default constant for Robin-Karp rolling hash. This is a random
// prime.
//
// TODO: What we would like to have is a prime in the range (2^63,2^64) with
// a Hamming weight around 31.
const A = 252097800623

// RabinKarp supports the computation of a rolling hash.
type RabinKarp struct {
	A uint64
	N int
	// a^{n-1}
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
	for i := 0; i < n-1; i++ {
		aOldest *= a
	}
	return &RabinKarp{A: a, aOldest: aOldest, N: n}
}

// AddYoung adds a "young" byte to the hash provided. The existing hash is
// shifted or multiplied accordingly.
func (r *RabinKarp) AddYoung(h uint64, b byte) uint64 {
	h *= r.A
	h += uint64(b)
	return h
}

// RemoveOldest removes the "oldest" byte from the hash. The hash value is not
// shifted or multiplied.
func (r *RabinKarp) RemoveOldest(h uint64, b byte) uint64 {
	h -= uint64(b) * r.aOldest
	return h
}

// Returns the length of the byte sequence this hash supports.
func (r *RabinKarp) Len() int {
	return r.N
}
