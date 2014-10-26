package lzma

import (
	"bufio"
	"errors"
	"io"
)

// states defines the overall state count
const states = 12

// bufferLen is the value for internal buffering of the decoder.
var bufferLen = 64 * (1 << 10)

// Decoder is able to read a LZMA byte stream and to read the plain text.
type Decoder struct {
	properties Properties
	packedLen  uint64
	total      uint64
	dict       *decoderDict
	state      uint32
	posBitMask uint32
	rd         *rangeDecoder
	match      [states << maxPosBits]prob
	rep        [states]prob
	repG0      [states]prob
	repG1      [states]prob
	repG2      [states]prob
	repG0Long  [states << maxPosBits]prob
}

// Properties returns a set of properties.
func (d *Decoder) Properties() Properties {
	return d.properties
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

// readUint64LE reads a uint64 little-endian integer from reader.
func readUint64LE(r io.Reader) (x uint64, err error) {
	b := make([]byte, 8)
	if _, err = io.ReadFull(r, b); err != nil {
		return 0, err
	}
	x = getUint64LE(b)
	return x, nil
}

func initProbSlice(p []prob) {
	for i := range p {
		p[i] = probInit
	}
}

// NewDecoder creates an LZMA decoder. It reads the classic, original LZMA
// format. Note that LZMA2 uses a different header format. It satisfies the
// io.Reader interface.
func NewDecoder(r io.Reader) (d *Decoder, err error) {
	f := bufio.NewReader(r)
	properties, err := readProperties(f)
	if err != nil {
		return nil, err
	}
	historyLen := int(properties.DictLen)
	if historyLen < 0 {
		return nil, errors.New(
			"LZMA property DictLen exceeds maximum int value")
	}
	d = &Decoder{
		properties: *properties,
	}
	if d.packedLen, err = readUint64LE(f); err != nil {
		return nil, err
	}
	if d.dict, err = newDecoderDict(bufferLen, historyLen); err != nil {
		return nil, err
	}
	d.posBitMask = (uint32(1) << uint(d.properties.PB)) - 1
	if d.rd, err = newRangeDecoder(f); err != nil {
		return nil, err
	}
	initProbSlice(d.match[:])
	initProbSlice(d.rep[:])
	initProbSlice(d.repG0[:])
	initProbSlice(d.repG1[:])
	initProbSlice(d.repG2[:])
	initProbSlice(d.repG0Long[:])
	return d, nil
}

// Reads reads data from the decoder stream.
//
// The function fill put as much data in the buffer as it is available. The
// function might block and is not reentrant.
//
// The end of the LZMA stream is indicated by EOF. There might be other errors
// returned. The decoder will not be able to recover from an error returned.
func (d *Decoder) Read(p []byte) (n int, err error) {
	for n < len(p) {
		var k int
		k, err = d.dict.Read(p)
		if err != nil {
			return
		}
		n += k
		if n == len(p) {
			return
		}
		if err = d.fill(len(p) - n); err != nil {
			return
		}
	}
	return
}

func (d *Decoder) fill(n int) error {
	panic("TODO")
}

func (d *Decoder) updateStateLiteral() {
	switch {
	case d.state < 4:
		d.state = 0
		return
	case d.state < 10:
		d.state -= 3
		return
	}
	d.state -= 6
	return
}

func (d *Decoder) updateStateMatch() {
	if d.state < 7 {
		d.state = 7
		return
	}
	d.state = 10
	return
}

func (d *Decoder) updateStateRep() {
	if d.state < 7 {
		d.state = 8
	}
	d.state = 11
}

func (d *Decoder) updateStateShortRep() {
	if d.state < 7 {
		d.state = 9
	}
	d.state = 11
}

func (d *Decoder) decodeOp() (op operation, err error) {
	posState := uint32(d.total) & d.posBitMask
	state2 := (d.state << maxPosBits) | posState

	b, err := d.match[state2].Decode(d.rd)
	if err != nil {
		return nil, err
	}
	if b == 0 {
		// literal
		panic("TODO")
	}
	b, err = d.rep[d.state].Decode(d.rd)
	if err != nil {
		return nil, err
	}
	if b == 0 {
		// simple match
		panic("TODO")
	}
	b, err = d.repG0[d.state].Decode(d.rd)
	if b == 0 {
		// rep0
		panic("TODO")
	}
	b, err = d.repG1[d.state].Decode(d.rd)
	if b == 0 {
		// rep match 1
		panic("TODO")
	}
	b, err = d.repG2[d.state].Decode(d.rd)
	if b == 0 {
		// rep match 2
		panic("TODO")
	}
	// rep match 3
	panic("TODO")
}
