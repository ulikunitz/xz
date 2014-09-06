package lzma

import (
	"bufio"
	"encoding/binary"
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

// UnmarshalBinary decodes properties in the old header format.
func (p *Properties) UnmarshalBinary(data []byte) error {
	x := int(data[0])
	p.LC = x % 9
	x /= 9
	p.LP = x % 5
	p.PB = x / 5
	if p.PB > 4 {
		return errors.New("pb property out of range")
	}
	p.DictSize = binary.LittleEndian.Uint32(data[1:5])
	return nil
}

// MarshalBinary encodes the properites as required by the old header format.
func (p *Properties) MarshalBinary() (data []byte, err error) {
	data = make([]byte, 5)
	b := (p.PB*5+p.LP)*9 + p.LC
	if !(0 <= b && b <= 0xff) {
		return nil, errors.New("invalid properties")
	}
	data[0] = byte(b)
	binary.LittleEndian.PutUint32(data[1:5], p.DictSize)
	return data, nil
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

// readOldHeader reads the complete old header from the reader. The return
// value size contains the uncompressed size.
func readOldHeader(r io.Reader) (props *Properties, size int64, err error) {
	buf := make([]byte, 13)
	if _, err = io.ReadFull(r, buf); err != nil {
		return nil, 0, err
	}
	props = new(Properties)
	if err = props.UnmarshalBinary(buf); err != nil {
		return nil, 0, err
	}
	s := binary.LittleEndian.Uint64(buf[5:])
	if s > math.MaxInt64 {
		return nil, 0, errors.New("size out of range")
	}
	return props, int64(s), err
}
