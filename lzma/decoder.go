package lzma

import (
	"bufio"
	"errors"
	"io"
)

// bufferLen is the value for internal buffering of the decoder.
var bufferLen = 64 * (1 << 10)

// Decoder is able to read a LZMA byte stream and to read the plain text.
type Decoder struct {
	properties Properties
	packedLen  uint64
	r          io.Reader
	dict       *decoderDict
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
		r:          f,
		properties: *properties,
	}
	if d.packedLen, err = readUint64LE(f); err != nil {
		return nil, err
	}
	if d.dict, err = newDecoderDict(bufferLen, historyLen); err != nil {
		return nil, err
	}
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
