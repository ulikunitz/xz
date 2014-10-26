package lzma

import (
	"errors"
	"io"
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
		return nil, errors.New("properties not read correctly")
	}
	p = new(Properties)
	x := int(b[0])
	p.LC = x % 9
	x /= 9
	p.LP = x % 5
	p.PB = x / 5
	if !(0 <= p.PB && p.PB <= 4) {
		return nil, errors.New("PB out of range")
	}
	p.DictLen = getUint32LE(b[1:])
	return p, nil
}
