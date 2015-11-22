package lzma

import (
	"errors"
	"fmt"
)

// Minimum and maximum values for the dictionary capacity that is called
// dictionary size by the LZMA specification.
const (
	minDictCap = 4096
	maxDictCap = 1<<32 - 1
)

// DecoderDict provides the dictionary to the Decoder. It provides a
// Read and a Write function to support the handling of uncompressed
// data.
type DecoderDict struct {
	buf      buffer
	head     int64
	capacity int
}

// NewDecoderDict creates a new decoder dictionary. The size of the
// allocated buffer will be the maximum of dictCap and bufSize. So
// bufSize indicates a minimum size for the buffer.
func NewDecoderDict(dictCap, bufSize int) (d *DecoderDict, err error) {
	// lower limit supports easy test cases
	if !(1 <= dictCap && int64(dictCap) <= maxDictCap) {
		return nil, errors.New("NewDecoderDict: dictCap out of range")
	}
	if dictCap > bufSize {
		bufSize = dictCap
	}
	d = &DecoderDict{capacity: dictCap}
	if err = initBuffer(&d.buf, bufSize); err != nil {
		return nil, err
	}
	return d, nil
}

// Reset clears the dictionary. The read buffer is not changed, so the
// buffered data can still be read.
func (d *DecoderDict) Reset() {
	d.head = 0
}

// WriteByte writes a single byte into the dictionary. It is used to
// write literals into the dictionary.
func (d *DecoderDict) WriteByte(c byte) error {
	if err := d.buf.WriteByte(c); err != nil {
		return err
	}
	d.head++
	return nil
}

// pos returns the position of the dictionary head.
func (d *DecoderDict) pos() int64 { return d.head }

// dictLen returns the actual length of the dictionary.
func (d *DecoderDict) dictLen() int {
	if d.head >= int64(d.capacity) {
		return d.capacity
	}
	return int(d.head)
}

// byteAt returns a byte stored in the dictionary. If the distance is
// non-positive or exceeds the current length of the dictionary the zero
// byte is returned.
func (d *DecoderDict) byteAt(dist int) byte {
	if !(0 < dist && dist <= d.dictLen()) {
		return 0
	}
	i := d.buf.front - dist
	if i < 0 {
		i += len(d.buf.data)
	}
	return d.buf.data[i]
}

// writeMatch writes the match at the top of the dictionary. The given
// distance must point in the current dictionary and the length must not
// exceed the maximum length 273 supported in LZMA.
//
// The error value ErrNoSpace indicates that no space is available in
// the dictionary for writing. You need to read from the dictionary
// first.
func (d *DecoderDict) writeMatch(dist int, length int) error {
	if !(0 < dist && dist <= d.dictLen()) {
		return errors.New("WriteMatch: distance out of range")
	}
	if !(0 < length && length <= maxMatchLen) {
		return errors.New("WriteMatch: length out of range")
	}
	if length > d.buf.Available() {
		return ErrNoSpace
	}
	d.head += int64(length)

	i := d.buf.front - dist
	if i < 0 {
		i += len(d.buf.data)
	}
	for length > 0 {
		var p []byte
		if i >= d.buf.front {
			p = d.buf.data[i:]
			i = 0
		} else {
			p = d.buf.data[i:d.buf.front]
			i = d.buf.front
		}
		if len(p) > length {
			p = p[:length]
		}
		if _, err := d.buf.Write(p); err != nil {
			panic(fmt.Errorf("Write returned error %s", err))
		}
		length -= len(p)
	}
	return nil
}

// Write writes the given bytes into the dictionary and advances the
// head.
func (d *DecoderDict) Write(p []byte) (n int, err error) {
	n, err = d.buf.Write(p)
	d.head += int64(n)
	return n, err
}

// Available returns the number of available bytes for writing into the
// decoder dictionary.
func (d *DecoderDict) Available() int { return d.buf.Available() }

// Read reads data from the buffer contained in the decoder dictionary.
func (d *DecoderDict) Read(p []byte) (n int, err error) { return d.buf.Read(p) }

// Buffered returns the number of bytes currently buffered in the
// decoder dictionary.
func (d *DecoderDict) buffered() int { return d.buf.Buffered() }

// Peek gets data from the buffer without advancing the rear index.
func (d *DecoderDict) peek(p []byte) (n int, err error) { return d.buf.Peek(p) }
