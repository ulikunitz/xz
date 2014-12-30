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

// verifyProperties checks properties for errors.
func verifyProperties(p *Properties) error {
	if p == nil {
		return newError("properties must be non-nil")
	}
	if !(MinLC <= p.LC && p.LC <= MaxLC) {
		return newError("LC out of range")
	}
	if !(MinLP <= p.LP && p.LP <= MaxLP) {
		return newError("LP out of range")
	}
	if !(MinPB <= p.PB && p.PB <= MaxPB) {
		return newError("PB out ouf range")
	}
	if !(MinDictLen <= p.DictLen && p.DictLen <= MaxDictLen) {
		return newError("DictLen out of range")
	}
	return nil
}

// getUint32LE reads an uint32 integer from a byte slize
func getUint32LE(b []byte) uint32 {
	x := uint32(b[3]) << 24
	x |= uint32(b[2]) << 16
	x |= uint32(b[1]) << 8
	x |= uint32(b[0])
	return x
}

// putUint32LE puts an uint32 integer into a byte slice that must have at least
// a lenght of 4 bytes.
func putUint32LE(b []byte, x uint32) {
	b[0] = byte(x)
	b[1] = byte(x >> 8)
	b[2] = byte(x >> 16)
	b[3] = byte(x >> 24)
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

// writeProperties writes properties to the stream. Note that properties are
// verified for out of range error to ensure that the properties can properly
// read from the stream again.
func writeProperties(w io.Writer, p *Properties) (err error) {
	if err = verifyProperties(p); err != nil {
		return err
	}
	b := make([]byte, 5)
	b[0] = byte((p.PB*5+p.LP)*9 + p.LC)
	putUint32LE(b[1:5], p.DictLen)
	_, err = w.Write(b)
	return err
}
