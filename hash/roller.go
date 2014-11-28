package hash

// Roller defines an interface provided by a rolling hash.
//
// The method Len provides the length of the byte sequences for which the
// rolling hash will be computed.
//
// The method AddYoung adds a new byte to the provided hash, whereby the hash
// value will be shifted or multiplied accordingly.
//
// The method RemoveOldest removes the provided oldest byte from the hash. The
// hash value will not be shifted or modified.
type Roller interface {
	Len() int
	AddYoung(h uint64, b byte) uint64
	RemoveOldest(h uint64, b byte) uint64
}

// ComputeHashes computes all hashes for the byte slices p using the rolling
// hash provided by r.
func ComputeHashes(r Roller, p []byte) []uint64 {
	m, n := len(p), r.Len()
	if m < n {
		return nil
	}
	h := make([]uint64, m-n+1)
	for i := 0; i < n; i++ {
		h[0] = r.AddYoung(h[0], p[i])
	}
	for i := 1; i < len(h); i++ {
		h[i] = r.RemoveOldest(h[i-1], p[i-1])
		h[i] = r.AddYoung(h[i], p[n-1+i])
	}
	return h
}
