package hash

// Roller provides an interface for rolling hashes.
type Roller interface {
	Hashes(p []byte) []uint64
}
