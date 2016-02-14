// Copyright 2014-2016 Ulrich Kunitz. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package lzma

import (
	"errors"
	"fmt"
	"io"
)

// maxMatches limits the number of matches requested from the Matches
// function. This controls the speed of the overall encoding.
const maxMatches = 16

// matcher is an interface that allows the identification of potential
// matches for words with a constant length greater or equal 2.
//
// The absolute offset of potential matches are provided by the
// Matches method.
//
// The Reset method clears the matcher completely but starts new data
// at the given position.
type matcher interface {
	io.Writer
	WordLen() int
	Matches(word []byte, positions []int64) int
	Reset()
}

// encoderDict provides the dictionary of the encoder. It includes an
// addtional buffer atop of the actual dictionary.
type encoderDict struct {
	buf        buffer
	m          matcher
	head       int64
	capacity   int
	shortDists int
	// preallocated arrays
	p         []int64
	distances []int
	wordSize  int
	data      []byte
}

// newEncoderDict creates the encoder dictionary. The argument bufSize
// defines the size of the additional buffer.
func newEncoderDict(dictCap, bufSize int) (d *encoderDict, err error) {
	const (
		shortDists = 8
		wordSize   = 4
	)

	if !(1 <= dictCap && int64(dictCap) <= MaxDictCap) {
		return nil, errors.New(
			"lzma: dictionary capacity out of range")
	}
	if bufSize < 1 {
		return nil, errors.New(
			"lzma: buffer size must be larger than zero")
	}
	d = &encoderDict{
		buf:        *newBuffer(dictCap + bufSize),
		capacity:   dictCap,
		p:          make([]int64, maxMatches),
		distances:  make([]int, 0, maxMatches+shortDists),
		shortDists: shortDists,
		wordSize:   wordSize,
		data:       make([]byte, maxMatchLen),
	}
	if d.m, err = newHashTable(dictCap, wordSize); err != nil {
		return nil, err
	}
	return d, nil
}

// NextOp computes the next operation for the encoding. It will provide
// always the longest match.
func (d *encoderDict) NextOp(rep0 uint32) operation {
	// get positions
	data := d.data[:maxMatchLen]
	n, _ := d.buf.Peek(data)
	data = data[:n]
	p := d.p
	if n < d.wordSize {
		p = p[:0]
	} else {
		n = d.m.Matches(data[:d.wordSize], p[:maxMatches])
		p = p[:n]
	}

	// convert positions in potential distances
	head := d.head
	dists := append(d.distances[:0], 1, 2, 3, 4, 5, 6, 7, 8)
	for _, pos := range p {
		dis := int(head - pos)
		if dis > d.shortDists {
			dists = append(dists, dis)
		}
	}

	// check distances
	var m match
	dictLen := d.DictLen()
	for _, dist := range dists {
		if dist > dictLen {
			continue
		}

		// Here comes a trick. We are only interested in matches
		// that are longer than the matches we have been found
		// before. So before we test the whole byte sequence at
		// the given distance, we test the first byte that would
		// make the match longer. If it doesn't match the byte
		// to match, we don't to care any longer.
		i := d.buf.rear - dist + m.n
		if i < 0 {
			i += len(d.buf.data)
		}
		if d.buf.data[i] != data[m.n] {
			// We can't get a longer match. Jump to the next
			// distance.
			continue
		}

		n := d.buf.matchLen(dist, data)
		switch n {
		case 0:
			continue
		case 1:
			if uint32(dist-minDistance) != rep0 {
				continue
			}
		}
		if n > m.n {
			m = match{int64(dist), n}
			if n == len(data) {
				// No better match will be found.
				break
			}
		}
	}
	if m.n == 0 {
		return lit{data[0]}
	}
	return m
}

// Discard discards n bytes. Note that n must not be larger than
// MaxMatchLen.
func (d *encoderDict) Discard(n int) {
	p := d.data[:n]
	k, _ := d.buf.Read(p)
	if k < n {
		panic(fmt.Errorf("lzma: can't discard %d bytes", n))
	}
	d.head += int64(n)
	d.m.Write(p)
}

// Len returns the data available in the encoder dictionary.
func (d *encoderDict) Len() int {
	n := d.buf.Available()
	if int64(n) > d.head {
		return int(d.head)
	}
	return n
}

// DictLen returns the actual length of data in the dictionary.
func (d *encoderDict) DictLen() int {
	if d.head < int64(d.capacity) {
		return int(d.head)
	}
	return d.capacity
}

// Available returns the number of bytes that can be written by a
// following Write call.
func (d *encoderDict) Available() int {
	return d.buf.Available() - d.DictLen()
}

// Write writes data into the dictionary buffer. Note that the position
// of the dictionary head will not be moved. If there is not enough
// space in the buffer ErrNoSpace will be returned.
func (d *encoderDict) Write(p []byte) (n int, err error) {
	m := d.Available()
	if len(p) > m {
		p = p[:m]
		err = ErrNoSpace
	}
	var e error
	if n, e = d.buf.Write(p); e != nil {
		err = e
	}
	return n, err
}

// Pos returns the position of the head.
func (d *encoderDict) Pos() int64 { return d.head }

// ByteAt returns the byte at the given distance.
func (d *encoderDict) ByteAt(distance int) byte {
	if !(0 < distance && distance <= d.Len()) {
		return 0
	}
	i := d.buf.rear - distance
	if i < 0 {
		i += len(d.buf.data)
	}
	return d.buf.data[i]
}

// CopyN copies the last n bytes from the dictionary into the provided
// writer. This is used for copying uncompressed data into an
// uncompressed segment.
func (d *encoderDict) CopyN(w io.Writer, n int) (written int, err error) {
	if n <= 0 {
		return 0, nil
	}
	m := d.Len()
	if n > m {
		n = m
		err = ErrNoSpace
	}
	i := d.buf.rear - n
	var e error
	if i < 0 {
		i += len(d.buf.data)
		if written, e = w.Write(d.buf.data[i:]); e != nil {
			return written, e
		}
		i = 0
	}
	var k int
	k, e = w.Write(d.buf.data[i:d.buf.rear])
	written += k
	if e != nil {
		err = e
	}
	return written, err
}

// Buffered returns the number of bytes in the buffer.
func (d *encoderDict) Buffered() int { return d.buf.Buffered() }
