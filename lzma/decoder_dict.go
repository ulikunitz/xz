package lzma

import (
	"fmt"
	"io"
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
	// provides total length
	total int64
	// marks eof
	eof bool
}

// newDecoderDict initializes a new decoderDict instance. If the arguments are
// negative or zero an error is returned.
func newDecoderDict(bufferLen int, historyLen int) (p *decoderDict, err error) {
	if !(0 < bufferLen) {
		return nil, newError("bufferLen must be positive")
	}
	if !(0 < historyLen) {
		return nil, newError("historyLen must be positive")
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
		return nil, newError(
			"LZMA dictionary size overflows integer range")
	}

	p = &decoderDict{
		data: make([]byte, 0, z),
		h:    historyLen,
		b:    bufferLen,
	}
	return p, nil
}

// reset clears the decoder dictionary. This function must be called by the
// LZMA2 code if the decoder dictionary needs to be reset without a change in
// parameters. A change in parameters requires the dictionary to be newly
// initialized.
func (p *decoderDict) reset() {
	p.data = p.data[:0]
	p.c = 0
	p.r = 0
	p.total = 0
	p.eof = false
}

// readable returns the number of bytes available for reading. If it is bigger
// then buffer length then decompression should stop.
func (p *decoderDict) readable() int {
	delta := p.c - p.r
	if delta >= 0 {
		return delta
	}
	return len(p.data) + delta
}

// writable returns the number of bytes that can be currently written to the
// dictionary. The dictionary needs to be read to increase the number. If the
// eof flag is set no bytes can be written.
func (p *decoderDict) writable() int {
	if p.eof {
		return 0
	}
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
// bytes. If the LZMA stream is finished it will return EOF  with no bytes
// read.
func (p *decoderDict) Read(a []byte) (n int, err error) {
	var k int
	if p.c < p.r {
		n = copy(a, p.data[p.r:])
		p.r += n
		if p.r > len(p.data) {
			goto out
		}
		p.r = 0
	}
	k = copy(a[n:], p.data[p.r:p.c])
	p.r += k
	n += k
out:
	if n == 0 && p.eof {
		return 0, io.EOF
	}
	return n, nil
}

// errOverflow indicates an overflow situation of the dictionary. It can be
// addressed by reading more data from the dictionary.
var errOverflow = newError("overflow")

// Write puts the complete slice in the dictionary or no bytes at all. If no
// bytes are written an overflow will be indicated. The slice b is now allowed
// to overlap with the p.data slice to write to. Note that an overflow error
// can also be caused by setting the EOF marker.
func (p *decoderDict) Write(b []byte) (n int, err error) {
	m := p.writable()
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
	p.total += int64(n)
	return n, nil
}

// AddByte adds a single byte to the decoder dictionary. Even here it is
// possible that the write will overflow.
func (p *decoderDict) addByte(b byte) error {
	_, err := p.Write([]byte{b})
	return err
}

// CopyMatch copies a match with the given length n and distance d.
func (p *decoderDict) copyMatch(d, n int) error {
	if n <= 0 {
		return newError("length n must be positive")
	}
	if d <= 0 {
		return newError("distance d must be positive")
	}
	if n > p.writable() {
		return errOverflow
	}
	if d > p.Len() {
		return newError("copyMatch argument d is too large")
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
			// incomplete transaction
			return newError(fmt.Sprintf(
				"p.Write unexpected error %s", err))
		}
		n -= k
		i += k
	}
	return nil
}

// setEOF sets the eof flag.
func (p *decoderDict) setEOF(eof bool) {
	p.eof = eof
}

// getByte returns the byte at distance d. If the distance is too large, the
// function returns zero.
func (p *decoderDict) getByte(d int) byte {
	if d <= 0 {
		panic("d must be positive")
	}
	if d > p.Len() {
		return 0
	}
	i := p.c - d
	if i < 0 {
		i += cap(p.data)
	}
	return p.data[i]
}
