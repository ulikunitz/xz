package lzma

import (
	"io"
)

// Maximum and minimum values for the individual properties.
const (
	MinLC      = 0
	MaxLC      = 8
	MinLP      = 0
	MaxLP      = 4
	MinPB      = 0
	MaxPB      = 4
	MinDictLen = 1 << 12
	MaxDictLen = 1<<32 - 1
)

// Properties provide the LZMA properties.
//
// Note that on 32-bit platforms not all possible dictionary length can be
// supported.
type Properties struct {
	// number of literal context bits
	LC int
	// number of literal position bits
	LP int
	// number of position bits
	PB int
	// length of the dictionary history in bytes
	DictLen uint32
}

// reads an uint32 integer from a byte slize
func getUint32LE(b []byte) uint32 {
	x := uint32(b[3]) << 24
	x |= uint32(b[2]) << 16
	x |= uint32(b[1]) << 8
	x |= uint32(b[0])
	return x
}

// readProperties reads the LZMA properties using the classic LZMA file header.
func readProperties(r io.Reader) (p *Properties, err error) {
	b := make([]byte, 5)
	n, err := io.ReadFull(r, b)
	if err != nil {
		return nil, err
	}
	if n != 5 {
		return nil, newError("properties not read correctly")
	}
	p = new(Properties)
	x := int(b[0])
	p.LC = x % 9
	x /= 9
	p.LP = x % 5
	p.PB = x / 5
	if !(MinPB <= p.PB && p.PB <= MaxPB) {
		return nil, newError("PB out of range")
	}
	p.DictLen = getUint32LE(b[1:])
	if p.DictLen < MinDictLen {
		// The LZMA specification makes the following recommendation.
		p.DictLen = MinDictLen
	}
	return p, nil
}
