package lzma

import (
	"io"
)

// Maximum and minimum values for individual properties.
const (
	MinLC       = 0
	MaxLC       = 8
	MinLP       = 0
	MaxLP       = 4
	MinPB       = 0
	MaxPB       = 4
	MinDictSize = 1 << 12
	MaxDictSize = 1<<32 - 1
)

// Properties are the parameters of an LZMA stream.
//
// The dictSize will be limited by MaxInt32 on 32-bit platforms.
type Properties struct {
	// number of literal context bits
	LC int
	// number of literal position bits
	LP int
	// number of position bits
	PB int
	// size of the dictionary in bytes
	DictSize uint32
	// size of uncompressed data
	Size int64
	// header includes unpacked size
	SizeInHeader bool
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
	if !(MinDictSize <= p.DictSize && p.DictSize <= MaxDictSize) {
		return newError("DictSize out of range")
	}
	hlen := int(p.DictSize)
	if hlen < 0 {
		return newError("DictSize cannot be converted into int")
	}
	if p.Size < 0 {
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
	p.DictSize = getUint32LE(b[1:])
	if p.DictSize < MinDictSize {
		// The LZMA specification makes the following recommendation.
		p.DictSize = MinDictSize
	}
	u := getUint64LE(b[5:])
	if u == noHeaderLen {
		p.Size = 0
		p.EOS = true
		p.SizeInHeader = false
		return p, nil
	}
	p.Size = int64(u)
	if p.Size < 0 {
		return nil, newError(
			"unpack length in header not supported by int64")
	}
	p.EOS = false
	p.SizeInHeader = true
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
	putUint32LE(b[1:5], p.DictSize)
	var l uint64
	if p.SizeInHeader {
		l = uint64(p.Size)
	} else {
		l = noHeaderLen
	}
	putUint64LE(b[5:], l)
	_, err = w.Write(b)
	return err
}
