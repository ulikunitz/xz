package lzma

import (
	"errors"
	"io"
)

// rangeEncoder implements the range encoding. The low value can overflow
// therefore we need uint64. The cache value is used to handle overflows.
type rangeEncoder struct {
	w         io.ByteWriter
	range_    uint32
	low       uint64
	cacheSize int64
	cache     byte
}

// newRangeEncoder creates a new range encoder.
func newRangeEncoder(w io.ByteWriter) *rangeEncoder {
	return &rangeEncoder{w: w, range_: 0xffffffff, cacheSize: 1}
}

// shiftLow() shifts the low value for 8 bit. The shifted byte is written into
// the byte writer. The cache value is used to handle overflows.
func (e *rangeEncoder) shiftLow() error {
	if uint32(e.low) < 0xff000000 || (e.low>>32) != 0 {
		tmp := e.cache
		for {
			err := e.w.WriteByte(tmp + byte(e.low>>32))
			if err != nil {
				return err
			}
			tmp = 0xff
			e.cacheSize--
			if e.cacheSize <= 0 {
				if e.cacheSize < 0 {
					panic("negative e.cacheSize")
				}
				break
			}
		}
		e.cache = byte(uint32(e.low) >> 24)
	}
	e.cacheSize++
	e.low = uint64(uint32(e.low) << 8)
	return nil
}

// normalize handles shifts of range_ and low.
func (e *rangeEncoder) normalize() error {
	const top = 1 << 24
	if e.range_ >= top {
		return nil
	}
	e.range_ <<= 8
	return e.shiftLow()
}

// directEncodeBit encodes the least-significant bit of b
func (e *rangeEncoder) directEncodeBit(b uint32) error {
	e.range_ >>= 1
	e.low += uint64(e.range_) & (0 - (uint64(b) & 1))
	return e.normalize()
}

// directEncode encodes the least-significant n bits. The most-significant bit
// will be encoded first.
func (e *rangeEncoder) directEncode(bits uint32, n int) error {
	for n--; n >= 0; n-- {
		if err := e.directEncodeBit(bits >> uint(n)); err != nil {
			return err
		}
	}
	return nil
}

// encodeBit encodes the least significant bit of b. The p value will be
// updated by the function depending on the bit encoded.
func (e *rangeEncoder) encodeBit(b uint32, p *prob) error {
	bound := p.bound(e.range_)
	if b&1 == 0 {
		e.range_ = bound
		p.inc()
	} else {
		e.low += uint64(bound)
		e.range_ -= bound
		p.dec()
	}
	return e.normalize()
}

// treeEncode encodes the p.bits least-significant bits of b starting with the
// most-significant bit.
func (e *rangeEncoder) treeEncode(b uint32, p *probTree) error {
	m := uint32(1)
	for i := p.bits - 1; i >= 0; i-- {
		x := (b >> uint(i)) & 1
		if err := e.encodeBit(x, &p.probs[m]); err != nil {
			return err
		}
		m = (m << 1) | x
	}
	return nil
}

// treeReverseEncode encodes the p.bits least-significant bits of b start with
// the least-signficant bit.
func (e *rangeEncoder) treeReverseEncode(b uint32, p *probTree) error {
	m := uint32(1)
	for i := 0; i < p.bits; i++ {
		x := (b >> uint(i)) & 1
		if err := e.encodeBit(x, &p.probs[m]); err != nil {
			return err
		}
		m = (m << 1) | x
	}
	return nil
}

// flush writes the complete low value out.
func (e *rangeEncoder) flush() error {
	for i := 0; i < 5; i++ {
		if err := e.shiftLow(); err != nil {
			return err
		}
	}
	return nil
}

// rangeDecoder decodes the range encoding stream.
type rangeDecoder struct {
	r      io.ByteReader
	range_ uint32
	code   uint32
}

// newRangeDecoder initializes a range decoder.  Note that the init() function
// must be used next.
func newRangeDecoder(r io.ByteReader) *rangeDecoder {
	return &rangeDecoder{r: r}
}

// init initializes the range decoder, by reading from the byte reader.
func (d *rangeDecoder) init() error {
	d.range_ = 0xffffffff
	d.code = 0

	b, err := d.r.ReadByte()
	if err != nil {
		return err
	}
	if b != 0 {
		return errors.New("first byte not zero")
	}

	for i := 0; i < 4; i++ {
		if err = d.updateCode(); err != nil {
			return err
		}
	}

	if d.code >= d.range_ {
		return errors.New("newRangeDecoder: d.code >= d.range_")
	}

	return nil
}

// finishingOk checks whether the code is zero.
func (d *rangeDecoder) finishingOk() bool {
	return d.code == 0
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
	// assume d.code < d.range_
	const top = 1 << 24
	if d.range_ < top {
		d.range_ <<= 8
		// d.code < d.range_ will be maintained
		if err := d.updateCode(); err != nil {
			return err
		}
	}
	return nil
}

// directDecodeBits decodes a bit directly. The return value b will contain the
// bit at the least-significant position. All other bits will be zero.
func (d *rangeDecoder) directDecodeBit() (b uint32, err error) {
	d.range_ >>= 1
	d.code -= d.range_
	t := 0 - (d.code >> 31)
	d.code += d.range_ & t

	// d.code will stay less then d.range_

	if err = d.normalize(); err != nil {
		return 0, err
	}
	return (t + 1) & 1, nil
}

// directDecode decodes n bits. The b value will contain those bits in the n
// least-significant positions. The most-significant bit will be decoded first.
func (d *rangeDecoder) directDecode(n int) (b uint32, err error) {
	for n--; n >= 0; n-- {
		a, err := d.directDecodeBit()
		if err != nil {
			return 0, err
		}
		b = (b << 1) | a
	}
	return b, nil
}

// decodeBit decodes a single bit. The bit will be returned at the
// least-significant position. The probability value will be updated.
func (d *rangeDecoder) decodeBit(p *prob) (b uint32, err error) {
	bound := p.bound(d.range_)
	if d.code < bound {
		d.range_ = bound
		p.inc()
		b = 0
	} else {
		d.code -= bound
		d.range_ -= bound
		p.dec()
		b = 1
	}

	// d.code will stay less then d.range_

	if err = d.normalize(); err != nil {
		return 0, err
	}
	return b, nil
}

// treeDecode decodes bits using the probTree. The number of bits is given b
// p.bits and the bits are decoded with highest-significant bits first.
func (d *rangeDecoder) treeDecode(p *probTree) (b uint32, err error) {
	m := uint32(1)
	for j := 0; j < p.bits; j++ {
		x, err := d.decodeBit(&p.probs[m])
		if err != nil {
			return 0, err
		}
		m = (m << 1) | x
	}
	return m - (1 << uint(p.bits)), nil
}

// treeReverseDecode decodes bits using the probTree. The number of bits is
// given b p.bits and the bits are decoded with least-significant bits first.
func (d *rangeDecoder) treeReverseDecode(p *probTree) (b uint32, err error) {
	m := uint32(1)
	for i := 0; i < p.bits; i++ {
		x, err := d.decodeBit(&p.probs[m])
		if err != nil {
			return 0, err
		}
		m = (m << 1) | x
		b |= (x << uint(i))
	}
	return b, nil
}
