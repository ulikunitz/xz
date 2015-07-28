// Copyright 2015 Ulrich Kunitz. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package lzma

import (
	"errors"
	"io"
)

const maxInt64 = 1<<63 - 1

// bWriter is used to convert a standard io.Writer into an io.ByteWriter.
type bWriter struct {
	io.Writer
	a []byte
}

// newByteWriter transforms an io.Writer into an io.ByteWriter.
func newByteWriter(w io.Writer) io.ByteWriter {
	if b, ok := w.(io.ByteWriter); ok {
		return b
	}
	return &bWriter{w, make([]byte, 1)}
}

// WriteByte writes a single byte into the Writer.
func (b *bWriter) WriteByte(c byte) error {
	b.a[0] = c
	n, err := b.Write(b.a)
	switch {
	case n > 1:
		panic("n > 1 for writing a single byte")
	case n == 1:
		return nil
	case err == nil:
		panic("no error for n == 0")
	}
	return err
}

// bReader is used to convert an io.Reader into an io.ByteReader.
type bReader struct {
	io.Reader
	a []byte
}

// newByteReader transforms an io.Reader into an io.ByteReader.
func newByteReader(r io.Reader) io.ByteReader {
	if b, ok := r.(io.ByteReader); ok {
		return b
	}
	return &bReader{r, make([]byte, 1)}
}

// ReadByte reads a byte from the wrapped io.ByteReader.
func (b bReader) ReadByte() (byte, error) {
	n, err := b.Read(b.a)
	switch {
	case n > 1:
		panic("n < 1 for reading a single byte")
	case n == 1:
		return b.a[0], nil
	}
	return 0, err
}

// rangeEncoder implements range encoding of single bits. The low value can
// overflow therefore we need uint64. The cache value is used to handle
// overflows.
type rangeEncoder struct {
	w        io.ByteWriter
	nrange   uint32
	low      uint64
	cacheLen int64
	cache    byte
	n        int64
	limit    int64
}

// newRangeEncoder creates a new range encoder.
func newRangeEncoder(w io.Writer) (re *rangeEncoder, err error) {
	return &rangeEncoder{
		w:        newByteWriter(w),
		nrange:   0xffffffff,
		cacheLen: 1}, nil
}

func newRangeEncoderLimit(w io.Writer, limit int64) (re *rangeEncoder, err error) {
	if limit < 0 {
		return nil, errors.New(
			"newRangeEncoderLimit: argument limit is negative")
	}
	if 0 < limit && limit < 5 {
		return nil, errors.New(
			"newRangeEncoderLimit: non-zero limit argument must " +
				"larger or equal 5")
	}
	re, err = newRangeEncoder(w)
	if err != nil {
		return nil, err
	}
	re.limit = limit
	return re, err
}

// Len returns the number of bytes actually written to the underlying
// writer.
func (re *rangeEncoder) Len() int64 {
	return re.n
}

// Available returns the number of bytes that still can be written. The
// method takes the bytes that will be currently written by Close into
// account.
func (re *rangeEncoder) Available() int64 {
	if re.limit == 0 {
		return maxInt64
	}
	return re.limit - (re.n + re.cacheLen + 4)
}

// writeByte writes a single byte to the underlying writer. An error is
// returned if the limit is reached. The written byte will be counted if
// the underlying writer doesn't return an error.
func (re *rangeEncoder) writeByte(c byte) error {
	if re.Available() < 1 {
		return errors.New("range encoder limit reached")
	}
	if err := re.w.WriteByte(c); err != nil {
		return err
	}
	re.n++
	return nil
}

// DirectEncodeBit encodes the least-significant bit of b with probability 1/2.
func (e *rangeEncoder) DirectEncodeBit(b uint32) error {
	// e.bitCounter++
	e.nrange >>= 1
	e.low += uint64(e.nrange) & (0 - (uint64(b) & 1))
	if err := e.normalize(); err != nil {
		return err
	}

	return nil
}

// EncodeBit encodes the least significant bit of b. The p value will be
// updated by the function depending on the bit encoded.
func (e *rangeEncoder) EncodeBit(b uint32, p *prob) error {
	// e.bitCounter++
	bound := p.bound(e.nrange)
	if b&1 == 0 {
		e.nrange = bound
		p.inc()
	} else {
		e.low += uint64(bound)
		e.nrange -= bound
		p.dec()
	}
	if err := e.normalize(); err != nil {
		return err
	}

	return nil
}

// Close writes a complete copy of the low value.
func (e *rangeEncoder) Close() error {
	for i := 0; i < 5; i++ {
		if err := e.shiftLow(); err != nil {
			return err
		}
	}
	return nil
}

// newRangeDecoder initializes a range decoder. It reads five bytes from the
// reader and therefore may return an error.
func newRangeDecoder(r io.Reader) (d *rangeDecoder, err error) {
	d = &rangeDecoder{r: newByteReader(r)}
	err = d.init()
	return
}

// possiblyAtEnd checks whether the decoder may be at the end of the stream.
func (d *rangeDecoder) possiblyAtEnd() bool {
	return d.code == 0
}

// DirectDecodeBit decodes a bit with probability 1/2. The return value b will
// contain the bit at the least-significant position. All other bits will be
// zero.
func (d *rangeDecoder) DirectDecodeBit() (b uint32, err error) {
	// d.bitCounter++
	d.nrange >>= 1
	d.code -= d.nrange
	t := 0 - (d.code >> 31)
	d.code += d.nrange & t

	// d.code will stay less then d.nrange

	if err = d.normalize(); err != nil {
		return 0, err
	}

	b = (t + 1) & 1

	return b, nil
}

// decodeBit decodes a single bit. The bit will be returned at the
// least-significant position. All other bits will be zero. The probability
// value will be updated.
func (d *rangeDecoder) DecodeBit(p *prob) (b uint32, err error) {
	// d.bitCounter++
	bound := p.bound(d.nrange)
	if d.code < bound {
		d.nrange = bound
		p.inc()
		b = 0
	} else {
		d.code -= bound
		d.nrange -= bound
		p.dec()
		b = 1
	}

	// d.code will stay less then d.nrange

	if err = d.normalize(); err != nil {
		return 0, err
	}

	return b, nil
}

// shiftLow shifts the low value for 8 bit. The shifted byte is written into
// the byte writer. The cache value is used to handle overflows.
func (e *rangeEncoder) shiftLow() error {
	if uint32(e.low) < 0xff000000 || (e.low>>32) != 0 {
		tmp := e.cache
		for {
			err := e.writeByte(tmp + byte(e.low>>32))
			if err != nil {
				return err
			}
			tmp = 0xff
			e.cacheLen--
			if e.cacheLen <= 0 {
				if e.cacheLen < 0 {
					panic(negError{"cacheLen", e.cacheLen})
				}
				break
			}
		}
		e.cache = byte(uint32(e.low) >> 24)
	}
	e.cacheLen++
	e.low = uint64(uint32(e.low) << 8)
	return nil
}

// normalize handles shifts of nrange and low.
func (e *rangeEncoder) normalize() error {
	const top = 1 << 24
	if e.nrange >= top {
		return nil
	}
	e.nrange <<= 8
	return e.shiftLow()
}

// rangeDecoder decodes single bits of the range encoding stream.
type rangeDecoder struct {
	r      io.ByteReader
	nrange uint32
	code   uint32
}

// init initializes the range decoder, by reading from the byte reader.
func (d *rangeDecoder) init() error {
	d.nrange = 0xffffffff
	d.code = 0

	b, err := d.r.ReadByte()
	if err != nil {
		return err
	}
	if b != 0 {
		return lzmaError{"first byte not zero"}
	}

	for i := 0; i < 4; i++ {
		if err = d.updateCode(); err != nil {
			return err
		}
	}

	if d.code >= d.nrange {
		return lzmaError{"newRangeDecoder: d.code >= d.nrange"}
	}

	return nil
}

// updateCode reads a new byte into the code.
func (d *rangeDecoder) updateCode() error {
	b, err := d.r.ReadByte()
	if err != nil {
		return err
	}
	d.code = (d.code << 8) | uint32(b)
	return nil
}

// normalize the top value and update the code value.
func (d *rangeDecoder) normalize() error {
	// assume d.code < d.nrange
	const top = 1 << 24
	if d.nrange < top {
		d.nrange <<= 8
		// d.code < d.nrange will be maintained
		if err := d.updateCode(); err != nil {
			return err
		}
	}
	return nil
}
