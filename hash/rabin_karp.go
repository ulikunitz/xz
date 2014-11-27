package hash

// A is the default constant for Robin-Karp rolling hash. This is a random
// prime.
//
// TODO: What we would like to have is a prime in the range (2^63,2^64) with
// a Hamming weight around 31.
const A = 252097800623

type RabinKarp struct {
	A uint64
	N int
	// a^{n-1}
	aOld uint64
}

func NewRabinKarp(n int) *RabinKarp {
	return NewRabinKarpConst(n, A)
}

func NewRabinKarpConst(n int, a uint64) *RabinKarp {
	if n <= 0 {
		panic("number of bytes n must be positive")
	}
	aOld := uint64(1)
	// There are faster methods. For the small n required by the LZMA
	// compressor O(n) is sufficient.
	for i := 0; i < n-1; i++ {
		aOld *= a
	}
	return &RabinKarp{A: a, aOld: aOld, N: n}
}

func (r *RabinKarp) NextHash(h uint64, bOld, bNew byte) uint64 {
	h -= uint64(bOld) * r.aOld
	h *= r.A
	h += uint64(bNew)
	return h
}

func (r *RabinKarp) Len() int {
	return r.N
}
