package rc

import "io"

type Bit byte

func (b Bit) Test() bool {
	return b&1 != 0
}

// moveBits defines the number of bits used for the updates of probability
// values.
const moveBits = 5

// ProbBits defines the number of bits of a probability value.
const ProbBits = 11

// Initial value for a probability value. It is 0.5.
const ProbInit Prob = 1 << (ProbBits - 1)

// Type Prob represents probabilities.
type Prob uint16

// Dec decreases the probability. The decrease is proportional to the
// probability value.
func (p *Prob) Dec() {
	*p -= *p >> moveBits
}

// Inc increases the probability. The Increase is proportional to the
// difference of 1 and the probability value.
func (p *Prob) Inc() {
	*p += ((1 << ProbBits) - *p) >> moveBits
}

// Computes the new bound for a given range using the probability value.
func (p Prob) Bound(r uint32) uint32 {
	return (r >> ProbBits) * uint32(p)
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

func (e *Encoder) EncodeDirect(b Bit) error {
	e.range_ >>= 1
	e.low += uint64(e.range_) & (0 - (uint64(b) & 1))
	return e.normalize()
}

func (e *Encoder) Encode(b Bit, p *Prob) error {
	bound := p.Bound(e.range_)
	if !b.Test() {
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
