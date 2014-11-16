package lzma

import (
	"errors"
	"fmt"
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
func newDecoderDict(bufferLen int, historyLen int) (p *decoderDict, err error) {
	if !(0 < bufferLen) {
		return nil, errors.New("bufferLen must be positive")
	}
	if !(0 < historyLen) {
		return nil, errors.New("historyLen must be positive")
	}

	// k maximum match length
	k := maxLength
	if historyLen < k {
		k = historyLen
	}
	z := historyLen
	// We want to be able to copy the whole history, which is limited by
	// the reader index.
	if z <= maxLength {
		z += 1
	}
	// We want to ensure that there is enough place for a match to have
	// more than bufferLen bytes readable.
	if bufferLen+k > z {
		z = bufferLen + k
	}

	// check for overflows
	if z < bufferLen || z < historyLen {
		return nil, errors.New(
			"LZMA dictionary size overflows integer range")
	}

	p = &decoderDict{
		data: make([]byte, 0, z),
		h:    historyLen,
		b:    bufferLen,
	}
	return p, nil
}

// Readable returns the number of bytes available for reading. If it is bigger
// then buffer length then decompression should stop.
func (p *decoderDict) Readable() int {
	delta := p.c - p.r
	if delta >= 0 {
		return delta
	}
	return len(p.data) + delta
}

// Writable returns the number of bytes that can be currently written to the
// dictionary. The dictionary needs to be read to increase the number.
func (p *decoderDict) Writable() int {
	delta := p.r - 1 - p.c
	if delta >= 0 {
		return delta
	}
	return cap(p.data) + delta
}

// Len returns the current length of the dictionary.
func (p *decoderDict) Len() int {
	if p.c > len(p.data) {
		panic("current index bigger then slice length")
	}
	if p.c == len(p.data) && p.c < p.h {
		return p.c
	}
	return p.h
}

// Read reads data from the dictionary into a. The function may return zero
// bytes.
func (p *decoderDict) Read(a []byte) (n int, err error) {
	if p.c < p.r {
		n = copy(a, p.data[p.r:])
		p.r += n
		if p.r < len(p.data) {
			return n, nil
		}
		p.r = 0
	}
	k := copy(a[n:], p.data[p.r:p.c])
	p.r += k
	n += k
	return n, nil
}

// errOverflow indicates an overflow situation of the dictionary. It can be
// addressed by reading more data from the dictionary.
var errOverflow = errors.New("overflow")

// Write puts the complete slice in the dictionary or no bytes at all. If no
// bytes are written an overflow will be indicated. The slice b is now allowed
// to overlap with the p.data slice to write to.
func (p *decoderDict) Write(b []byte) (n int, err error) {
	m := p.Writable()
	n = len(b)
	if n > m {
		return 0, errOverflow
	}
	c := p.c + n
	z := cap(p.data)
	k := c
	if c > z {
		k = z
	}
	if len(p.data) < k {
		p.data = p.data[:k]
	}
	a := copy(p.data[p.c:], b)
	if a < n {
		p.c = copy(p.data, b[a:])
	} else {
		p.c = c
	}
	return n, nil
}

// AddByte adds a single byte to the decoder dictionary. Even here it is
// possible that the write will overflow.
func (p *decoderDict) AddByte(b byte) error {
	_, err := p.Write([]byte{b})
	return err
}

// CopyMatch copies a match with the given length n and distance d.
func (p *decoderDict) CopyMatch(d, n int) error {
	if n <= 0 {
		return errors.New("argument n must be positive")
	}
	if d <= 0 {
		return errors.New("argument d must be positive")
	}
	if n > p.Writable() {
		return errOverflow
	}
	if d > p.Len() {
		return errors.New("argument d is to large")
	}
	z := cap(p.data)
	i := p.c - d
	for n > 0 {
		a, b := i, i+n
		if b > p.c {
			b = p.c
		}
		if a < 0 {
			a, b = a+z, b+z
		}
		if b > z {
			b = z
		}
		k, err := p.Write(p.data[a:b])
		if err != nil {
			// Must panic here, because we don't want an incomplete
			// transaction
			panic(fmt.Sprintf("p.Write unexpected error %s", err))
		}
		n -= k
		i += k
	}
	return nil
}

// getByte returns the byte at distance d. If the distance is too large, the
// function returns zero.
func (p *decoderDict) getByte(d int) byte {
	if d < 0 {
		panic("negative d unexpected")
	}
	if d >= p.Len() {
		return 0
	}
	i := p.c - d
	if i < 0 {
		i += cap(p.data)
	}
	return p.data[i]
}
