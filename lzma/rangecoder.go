package lzma

import (
	"errors"
	"io"
)

// A bit represents a single bit. The bit is set if the values not zero.
type bit byte

// test tests whether the bit is set.
func (b bit) test() bool {
	return b != 0
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
func (p *prob) Dec() {
	*p -= *p >> movebits
}

// Inc increases the probability. The Increase is proportional to the
// difference of 1 and the probability value.
func (p *prob) Inc() {
	*p += ((1 << probbits) - *p) >> movebits
}

// Computes the new bound for a given range using the probability value.
func (p prob) Bound(r uint32) uint32 {
	return (r >> probbits) * uint32(p)
}

type Encoder struct {
	w         io.ByteWriter
	range_    uint32
	low       uint64
	cacheSize int64
	cache     byte
}

func NewEncoder(w io.ByteWriter) *Encoder {
	return &Encoder{w: w, range_: 0xffffffff, cacheSize: 1}
}

func (e *Encoder) shiftLow() error {
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

func (e *Encoder) normalize() error {
	const top = 1 << 24
	if e.range_ >= top {
		return nil
	}
	e.range_ <<= 8
	return e.shiftLow()
}

func (e *Encoder) EncodeDirect(b bit) error {
	e.range_ >>= 1
	e.low += uint64(e.range_) & (0 - (uint64(b) & 1))
	return e.normalize()
}

func (e *Encoder) Encode(b bit, p *prob) error {
	bound := p.Bound(e.range_)
	if !b.test() {
		e.range_ = bound
		p.Inc()
	} else {
		e.low += uint64(bound)
		e.range_ -= bound
		p.Dec()
	}
	return e.normalize()
}

func (e *Encoder) Flush() error {
	for i := 0; i < 5; i++ {
		if err := e.shiftLow(); err != nil {
			return err
		}
	}
	return nil
}

type Decoder struct {
	r      io.ByteReader
	range_ uint32
	code   uint32
}

func NewDecoder(r io.ByteReader) *Decoder {
	return &Decoder{r: r}
}

func (d *Decoder) Init() error {
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

func (d *Decoder) finishingOk() bool {
	return d.code == 0
}

func (d *Decoder) updateCode() error {
	b, err := d.r.ReadByte()
	if err != nil {
		return err
	}
	d.code = (d.code << 8) | uint32(b)
	return nil
}

func (d *Decoder) normalize() error {
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

func (d *Decoder) DecodeDirect() (b bit, err error) {
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

func (d *Decoder) Decode(p *prob) (b bit, err error) {
	bound := p.Bound(d.range_)
	if d.code < bound {
		d.range_ = bound
		p.Inc()
		b = 0
	} else {
		d.code -= bound
		d.range_ -= bound
		p.Dec()
		b = 1
	}

	// d.code will stay less then d.range_

	if err = d.normalize(); err != nil {
		return 0, err
	}
	return b, nil
}
