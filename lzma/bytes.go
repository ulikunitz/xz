package lzma

// putLE32 write a uint32 value into the slice p using little endian encoding.
// The p slice must have at least length four.
func putLE32(p []byte, x uint32) {
	_ = p[3]
	p[0] = byte(x)
	p[1] = byte(x >> 8)
	p[2] = byte(x >> 16)
	p[3] = byte(x >> 24)
}

// getLE32 reads a uint32 value from the slice p. Slice p must have at least
// length 4.
func getLE32(p []byte) uint32 {
	_ = p[3]
	var x uint32
	x = uint32(p[0])
	x |= uint32(p[1]) << 8
	x |= uint32(p[2]) << 16
	x |= uint32(p[3]) << 24
	return x
}

// putLE64 writes a uint64 value into the slice p using little endian encoding.
// The length of slice p must be at least 8 bytes.
func putLE64(p []byte, x uint64) {
	_ = p[7]
	p[0] = byte(x)
	p[1] = byte(x >> 8)
	p[2] = byte(x >> 16)
	p[3] = byte(x >> 24)
	p[4] = byte(x >> 32)
	p[5] = byte(x >> 40)
	p[6] = byte(x >> 48)
	p[7] = byte(x >> 56)
}

// getLE64 reads a uint64 value from the slice p using little endian encoding.
// The length of p must be at least 8 bytes.
func getLE64(p []byte) uint64 {
	_ = p[7]
	var x uint64
	x = uint64(p[0])
	x |= uint64(p[1]) << 8
	x |= uint64(p[2]) << 16
	x |= uint64(p[3]) << 24
	x |= uint64(p[4]) << 32
	x |= uint64(p[5]) << 40
	x |= uint64(p[6]) << 48
	x |= uint64(p[7]) << 56
	return x
}

func getBE16(p []byte) uint16 {
	_ = p[1]
	return uint16(p[0])<<8 | uint16(p[1])
}
