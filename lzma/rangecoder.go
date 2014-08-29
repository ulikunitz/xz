package lzma

import (
	"errors"
	"io"
)

// A bit represents a single bit. The bit is set if the values not zero.
type bit byte

// test tests whether the bit is set.
func (b bit) test() bool {
	return b&1 != 0
}

// movebits defines the number of bits used for the updates of probability
// values.
const movebits = 5

// probbits defines the number of bits of a probability value.
const probbits = 11

// probInit defines 0.5 as initial value for prob values.
const probInit prob = 1 << (probbits - 1)

// Type prob represents probabilities.
type prob uint16

// Dec decreases the probability. The decrease is proportional to the
// probability value.
func (p *prob) dec() {
	*p -= *p >> movebits
}

// Inc increases the probability. The Increase is proportional to the
// difference of 1 and the probability value.
func (p *prob) inc() {
	*p += ((1 << probbits) - *p) >> movebits
}

// Computes the new bound for a given range using the probability value.
func (p prob) bound(r uint32) uint32 {
	return (r >> probbits) * uint32(p)
}

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

// encodeDirect encodes the bit directly.
func (e *rangeEncoder) encodeDirect(b bit) error {
	e.range_ >>= 1
	e.low += uint64(e.range_) & (0 - (uint64(b) & 1))
	return e.normalize()
}

// encodes the bit using the given probability which is updated according to
// the bit value.
func (e *rangeEncoder) encode(b bit, p *prob) error {
	bound := p.bound(e.range_)
	if !b.test() {
		e.range_ = bound
		p.inc()
	} else {
		e.low += uint64(bound)
		e.range_ -= bound
		p.dec()
	}
	return e.normalize()
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

// decodeDirect decodes a bit directly.
func (d *rangeDecoder) decodeDirect() (b bit, err error) {
	d.range_ >>= 1
	d.code -= d.range_
	t := 0 - (d.code >> 31)
	d.code += d.range_ & t

	// d.code will stay less then d.range_

	if err = d.normalize(); err != nil {
		return 0, err
	}
	return bit((t + 1) & 1), nil
}

// decode decodes a single bit. The probability value will be updated.
func (d *rangeDecoder) decode(p *prob) (b bit, err error) {
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
