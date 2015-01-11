package lzma

import (
	"io"
)

// Maximum and minimum values for individual properties.
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

// Properties are the parameters of an LZMA stream.
//
// The dictLen will be limited to MaxInt32 on 32-bit platforms.
type Properties struct {
	// number of literal context bits
	LC int
	// number of literal position bits
	LP int
	// number of position bits
	PB int
	// length of the dictionary history in bytes
	DictLen uint32
	// length of uncompressed data
	Len int64
	// header includes unpacked length
	LenInHeader bool
	// end-of-stream marker requested
	EOS bool
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
	hlen := int(p.DictLen)
	if hlen < 0 {
		return newError("DictLen cannot be converted into int")
	}
	if p.Len < 0 {
		return newError("length must not be negative")
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

// getUint64LE converts the uint64 value stored as little endian to an uint64
// value.
func getUint64LE(b []byte) uint64 {
	x := uint64(b[7]) << 56
	x |= uint64(b[6]) << 48
	x |= uint64(b[5]) << 40
	x |= uint64(b[4]) << 32
	x |= uint64(b[3]) << 24
	x |= uint64(b[2]) << 16
	x |= uint64(b[1]) << 8
	x |= uint64(b[0])
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

// putUint64LE puts the uint64 value into the byte slice as little endian
// value. The byte slice b must have at least place for 8 bytes.
func putUint64LE(b []byte, x uint64) {
	b[0] = byte(x)
	b[1] = byte(x >> 8)
	b[2] = byte(x >> 16)
	b[3] = byte(x >> 24)
	b[4] = byte(x >> 32)
	b[5] = byte(x >> 40)
	b[6] = byte(x >> 48)
	b[7] = byte(x >> 56)
}

// readHeader reads the classic LZMA header.
func readHeader(r io.Reader) (p *Properties, err error) {
	b := make([]byte, 13)
	_, err = io.ReadFull(r, b)
	if err != nil {
		return nil, err
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
	u := getUint64LE(b[5:])
	if u == noHeaderLen {
		p.Len = 0
		p.EOS = true
		p.LenInHeader = false
		return p, nil
	}
	p.Len = int64(u)
	if p.Len < 0 {
		return nil, newError(
			"unpack length in header not supported by int64")
	}
	p.EOS = false
	p.LenInHeader = true
	return p, nil
}

// writeHeader writes the header for classic LZMA files.
func writeHeader(w io.Writer, p *Properties) error {
	var err error
	if err = verifyProperties(p); err != nil {
		return err
	}
	b := make([]byte, 13)
	b[0] = byte((p.PB*5+p.LP)*9 + p.LC)
	putUint32LE(b[1:5], p.DictLen)
	var l uint64
	if p.LenInHeader {
		l = uint64(p.Len)
	} else {
		l = noHeaderLen
	}
	putUint64LE(b[5:], l)
	_, err = w.Write(b)
	return err
}
