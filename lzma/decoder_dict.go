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

// Readable returns the number of bytes available for reading. If it is bigger
// then buffer length then decompression should stop.
func (d *decoderDict) Readable() int {
	delta := d.c - d.r
	if delta < 0 {
		return len(d.data) + delta
	}
	return delta
}

// Writable returns the number of bytes that are currently writable.
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

// Writes the complete slice or no bytes at all. If no bytes are written an
// overflow will be indicated. The slice b is now allowed to overlap with
// the d.data slice to write to.
func (d *decoderDict) Write(b []byte) (n int, err error) {
	delta := d.r - 1 - d.c
	n = len(b)
	if delta >= 0 {
		if n > delta {
			return 0, errOverflow
		}
		k := copy(d.data[d.c:], b)
		// Optimize: test should be removed
		if k != n {
			panic("unexpected copy result")
		}
		d.c += n
		return n, nil
	}
	z := cap(d.data)
	if n > z+delta {
		return 0, errOverflow
	}
	l := len(d.data)
	c := d.c + n
	if c <= z {
		if c > l {
			d.data = d.data[:c]
		}
		k := copy(d.data[d.c:], b)
		// Optimize: test should be removed
		if k != n {
			panic("unexpected copy result")
		}
		d.c = c % z
		return n, nil
	}
	if z > l {
		d.data = d.data[:z]
	}
	m := copy(d.data[d.c:], b)
	// Optimize: test should be removed
	if m != z-d.c {
		panic("unexpected copy result")
	}
	d.c = copy(d.data[0:], b[m:])
	// Optimize: test should be removed
	if d.c != c-z {
		panic("unexpected copy result")
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
