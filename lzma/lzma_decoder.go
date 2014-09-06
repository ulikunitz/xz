package lzma

import (
	"bufio"
	"errors"
	"io"
	"math"
)

type Decoder struct {
}

// Properties stores the properties of a range decoder
type Properties struct {
	// "literal context" bits [0,8]
	LC int
	// "literal pos" bits [0,4]
	LP int
	// "pos" bits [0,4]
	PB int
	// dictSize [0,2^32-1]
	DictSize uint32
}

func (d *Decoder) DictionarySize() int64 {
	panic("TODO")
}

func (d *Decoder) SetDictionarySize(n int64) error {
	panic("TODO")
}

func (d *Decoder) ResetState() {
	panic("TODO")
}

func (d *Decoder) ResetDictionary() {
	panic("TODO")
}

func (d *Decoder) SetProperties(p Properties) {
	panic("TODO")
}

var ErrInvalidHeader = errors.New("invalid header")

// NewDecoder initializes a decoder for the classic LZMA format with the long
// header. The function reads the header, which may cause an ErrInvalidHeader
// or other error.
func NewDecoder(r io.Reader) (d *Decoder, err error) {
	panic("TODO")
}

// makeByteReader makes a byte reader out of an io.Reader. It checks whether
// the interface is already an io.ByteReader or wraps it in a bufio Reader.
func makeByteReader(r io.Reader) io.ByteReader {
	if b, ok := r.(io.ByteReader); ok {
		return b
	}
	return bufio.NewReader(r)
}

func decodeUint32LE(b []byte) uint32 {
	u := uint32(b[0])
	u |= uint32(b[1]) << 8
	u |= uint32(b[2]) << 16
	u |= uint32(b[3]) << 24
	return u
}

func decodeUint64LE(b []byte) uint64 {
	u := uint64(b[0])
	u |= uint64(b[1]) << 8
	u |= uint64(b[2]) << 16
	u |= uint64(b[3]) << 24
	u |= uint64(b[4]) << 32
	u |= uint64(b[5]) << 40
	u |= uint64(b[6]) << 48
	u |= uint64(b[7]) << 56
	return u
}

func decodeProperties(b []byte) (p *Properties, err error) {
	p = new(Properties)
	x := int(b[0])
	p.LC = x % 9
	x /= 9
	p.LP = x % 5
	p.PB = x / 5
	if p.PB > 4 {
		return nil, errors.New("pb property out of range")
	}
	p.DictSize = decodeUint32LE(b[1:5])
	return p, nil
}

func readOldHeader(r io.Reader) (props *Properties, size int64, err error) {
	buf := make([]byte, 13)
	if _, err = io.ReadFull(r, buf); err != nil {
		return nil, 0, err
	}
	if props, err = decodeProperties(buf); err != nil {
		return nil, 0, err
	}
	s := decodeUint64LE(buf[5:])
	if s > math.MaxInt64 {
		return nil, 0, errors.New("size out of range")
	}
	return props, int64(s), err
}
