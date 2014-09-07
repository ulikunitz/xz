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

// Properties stores the properties of a range decoder. Note that we only
// support dictionary size up to MaxInt. On x86 this is smaller than the
// maximum 2^32-1 supported by the LZMA specification.
type Properties struct {
	// "literal context" bits [0,8]
	LC int
	// "literal pos" bits [0,4]
	LP int
	// "pos" bits [0,4]
	PB int
	// dictSize [0,2^32-1]
	DictSize int
}

// The minimum dictionary size used.
const MinDictSize = 1 << 12

// unmarshal decodes properties in the old header format. If the dictionary
// size is less then 2^12, MinDictSize, it is set to it as defined in the draft
// LZMA specification.
func (p *Properties) unmarshal(data []byte) error {
	x := int(data[0])
	p.LC = x % 9
	x /= 9
	p.LP = x % 5
	p.PB = x / 5
	if p.PB > 4 {
		return errors.New("pb property out of range")
	}
	p.DictSize = int(binary.LittleEndian.Uint32(data[1:5]))
	if p.DictSize < 0 {
		return errors.New("dictionary size out of range")
	}
	if p.DictSize < MinDictSize {
		p.DictSize = MinDictSize
	}
	return nil
}

// marshal encodes the properites as required by the old header format.
func (p *Properties) marshal(data []byte) (err error) {
	b := (p.PB*5+p.LP)*9 + p.LC
	if !(0 <= b && b <= 0xff) {
		return errors.New("invalid properties")
	}
	data[0] = byte(b)
	if !(0 <= p.DictSize && p.DictSize <= math.MaxUint32) {
		return errors.New("dict size out of range")
	}
	u := uint32(p.DictSize)
	binary.LittleEndian.PutUint32(data[1:5], u)
	return nil
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

const oldHeaderLen = 13

type oldHeader struct {
	Properties
	size int64
}

func (o *oldHeader) unmarshal(data []byte) (err error) {
	if err = o.Properties.unmarshal(data); err != nil {
		return err
	}
	s := binary.LittleEndian.Uint64(data[5:])
	if s > math.MaxInt64 {
		return errors.New("size out of range")
	}
	o.size = int64(s)
	return nil
}

func (o *oldHeader) read(r io.Reader) (err error) {
	data := make([]byte, oldHeaderLen)
	if _, err = io.ReadFull(r, data); err != nil {
		return err
	}
	return o.unmarshal(data)
}
