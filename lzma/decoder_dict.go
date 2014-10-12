package lzma

import (
	"errors"
)

// decoderDict represents the dictionary while decoding LZMA files. It allows
// reading and decompression of the data at the same time.
type decoderDict struct {
	data []byte
	// history length
	h int
	// buffer length
	b int
	// current index
	c int
	// reader index
	r int
}

// newDecoderDict initializes a new decoderDict instance. If the arguments are
// negative or zero an error is returned.
func newDecoderDict(bufferLen int, historyLen int) (d *decoderDict, err error) {
	if !(0 < bufferLen) {
		return nil, errors.New("bufferLen must be positive")
	}
	if !(0 < historyLen) {
		return nil, errors.New("historyLen must be positive")
	}

	z := historyLen
	// We want to be able to copy the whole history, which is limited by
	// the reader index.
	if z <= maxLength {
		z += 1
	}
	if bufferLen > z {
		z = bufferLen
	}

	d = &decoderDict{
		data: make([]byte, 0, z),
		h:    historyLen,
		b:    bufferLen,
	}
	return d, nil
}

// Readable returns the number of bytes available for reading. If it is bigger
// then buffer length then decompression should stop.
func (d *decoderDict) Readable() int {
	delta := d.c - d.r
	if delta >= 0 {
		return delta
	}
	return len(d.data) + delta
}

// Writable returns the number of bytes that can be currently written to the
// dictionary. The dictionary needs to be read to increase the number.
func (d *decoderDict) Writable() int {
	delta := d.r - 1 - d.c
	if delta >= 0 {
		return delta
	}
	return cap(d.data) + delta
}

// Len returns the current length of the dictionary.
func (d *decoderDict) Len() int {
	if d.c > len(d.data) {
		panic("current index bigger then slice length")
	}
	if d.c == len(d.data) && d.c < d.h {
		return d.c
	}
	return d.h
}

// Read reads data from the dictionary into p. The function may return zero
// bytes.
func (d *decoderDict) Read(p []byte) (n int, err error) {
	if d.c < d.r {
		n = copy(p, d.data[d.r:])
		d.r += n
		if d.r < len(d.data) {
			return n, nil
		}
		d.r = 0
	}
	k := copy(p[n:], d.data[d.r:d.c])
	d.r += k
	n += k
	return n, nil
}

// errOverflow indicates an overflow situation of the dictionary. It can be
// addressed by reading more data from the dictionary.
var errOverflow = errors.New("overflow")

// Write puts the complete slice in the dictionary or no bytes at all. If no
// bytes are written an overflow will be indicated. The slice b is now allowed
// to overlap with the d.data slice to write to.
func (d *decoderDict) Write(b []byte) (n int, err error) {
	m := d.Writable()
	n = len(b)
	if n > m {
		return 0, errOverflow
	}
	c := d.c + n
	z := cap(d.data)
	k := c
	if c > z {
		k = z
	}
	if len(d.data) < k {
		d.data = d.data[:k]
	}
	a := copy(d.data[d.c:], b)
	if a < n {
		d.c = copy(d.data, b[a:])
	} else {
		d.c = c
	}
	return n, nil
}

// AddByte adds a single byte to the decoder dictionary. Even here it is
// possible that the write will overflow.
func (d *decoderDict) AddByte(b byte) error {
	_, err := d.Write([]byte{b})
	return err
}

func (d *decoderDict) CopyMatch(length int, distance int) error {
	panic("TODO")
}
