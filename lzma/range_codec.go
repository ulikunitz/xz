package lzma

import (
	"errors"
	"io"
	"math/bits"
)

// moveBits defines the number of bits used for the updates of probability
// values.
const moveBits = 5

// probBits defines the number of bits of a probability value.
const probBits = 11

// probInit defines 0.5 as initial value for prob values.
const probInit prob = 1 << (probBits - 1)

// Type prob represents probabilities. The type can also be used to encode and
// decode single bits.
type prob uint16

// IncProb increases the probability. The Increase is proportional to the
// difference of 1 and the probability value.
func incProb(p prob) prob {
	return p + ((1<<probBits)-p)>>moveBits
}

// decProb decreases the probability. The decrease is proportional to the
// probability value.
func decProb(p prob) prob {
	return p - p>>moveBits
}

// Computes the new bound for a given range using the probability value.
func (p prob) bound(r uint32) uint32 {
	return (r >> probBits) * uint32(p)
}

type rEncoder interface {
	directEncodeBit(b uint32) error
	encodeBit(b uint32, p *prob) error
	Close() error
}

type rangeBitCounter struct {
	_bits  int64
	low    uint64
	nrange uint32
}

func (c *rangeBitCounter) init() {
	*c = rangeBitCounter{
		nrange: 1<<32 - 1,
	}
}

func (c *rangeBitCounter) fromRangeEncoder(e *rangeEncoder) {
	*c = rangeBitCounter{
		low:    e.low,
		nrange: e.nrange,
	}
}


func (c *rangeBitCounter) directEncodeBit(b uint32) error {
	c.nrange >>= 1
	c.low += uint64(c.nrange) & (0 - (uint64(b) & 1))

	// normalize
	const top = 1 << 24
	if c.nrange >= top {
		return nil
	}
	c.nrange <<= 8
	return c.shiftLow()
}

func (c *rangeBitCounter) encodeBit(b uint32, p *prob) error {
	nrange := c.nrange
	bound := p.bound(nrange)
	if b&1 == 0 {
		nrange = bound
		*p = incProb(*p)
	} else {
		c.low += uint64(bound)
		nrange -= bound
		*p = decProb(*p)
	}

	// normalize
	if nrange >= (1 << 24) {
		c.nrange = nrange
		return nil
	}
	c.nrange = nrange << 8
	return c.shiftLow()
}

func (c *rangeBitCounter) Close() error {
	for range 5 {
		if err := c.shiftLow(); err != nil {
			return err
		}
	}
	// corrects the output of bits
	c.nrange = 1<<32 - 1
	return nil
}

func (c *rangeBitCounter) shiftLow() error {
	c._bits += 8
	c.low = uint64(uint32(c.low) << 8)
	return nil
}

func (c *rangeBitCounter) bits() int64 {
	n := c._bits
	n += int64(bits.LeadingZeros32(c.nrange))
	return n
}

// rangeEncoder implements range encoding of single bits. The low value can
// overflow therefore we need uint64. The cache value is used to handle
// overflows.
type rangeEncoder struct {
	bw       io.ByteWriter
	low      uint64
	cacheLen int
	nrange   uint32
	cache    byte
}

// init initializes the range encoder
func (e *rangeEncoder) init(bw io.ByteWriter) {
	*e = rangeEncoder{
		bw:       bw,
		nrange:   1<<32 - 1,
		cacheLen: 1,
	}
}

// directEncodeBit encodes the least-significant bit of b with probability 1/2.
func (e *rangeEncoder) directEncodeBit(b uint32) error {
	e.nrange >>= 1
	e.low += uint64(e.nrange) & (0 - (uint64(b) & 1))

	// normalize
	const top = 1 << 24
	if e.nrange >= top {
		return nil
	}
	e.nrange <<= 8
	return e.shiftLow()
}

// encodeBit encodes the least significant bit of b. The p value will be
// updated by the function depending on the bit encoded.
func (e *rangeEncoder) encodeBit(b uint32, p *prob) error {
	nrange := e.nrange
	bound := p.bound(nrange)
	if b&1 == 0 {
		nrange = bound
		*p = incProb(*p)
	} else {
		e.low += uint64(bound)
		nrange -= bound
		*p = decProb(*p)
	}

	// normalize
	if nrange >= (1 << 24) {
		e.nrange = nrange
		return nil
	}
	e.nrange = nrange << 8
	return e.shiftLow()
}

// Close writes a complete copy of the low value.
func (e *rangeEncoder) Close() error {
	for range 5 {
		if err := e.shiftLow(); err != nil {
			return err
		}
	}
	return nil
}

// shiftLow shifts the low value for 8 bit. The shifted byte is written into
// the byte writer. The cache value is used to handle overflows.
func (e *rangeEncoder) shiftLow() error {
	if uint32(e.low) < 0xff000000 || (e.low>>32) != 0 {
		tmp := e.cache
		for {
			err := e.bw.WriteByte(tmp + byte(e.low>>32))
			if err != nil {
				return err
			}
			tmp = 0xff
			e.cacheLen--
			if e.cacheLen <= 0 {
				if e.cacheLen < 0 {
					panic("negative cacheLen")
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

// rangeDecoder decodes single bits of the range encoding stream.
type rangeDecoder struct {
	br     io.ByteReader
	nrange uint32
	code   uint32
}

// init initializes the rangeDecoder. It reads five bytes from the stream and
// may return errors.
func (d *rangeDecoder) init(br io.ByteReader) error {
	*d = rangeDecoder{br: br, nrange: 0xffffffff}

	b, err := d.br.ReadByte()
	if err != nil {
		return err
	}
	if b != 0 {
		return errors.New("lzma: first byte of LZMA stream not zero")
	}
	for range 4 {
		if err = d.updateCode(); err != nil {
			return err
		}
	}
	if d.code >= d.nrange {
		return errors.New("lzma: d.code >= d.nrange")
	}
	return nil
}

// possiblyAtEnd checks whether the decoder may be at the end of the stream.
func (d *rangeDecoder) possiblyAtEnd() bool {
	return d.code == 0
}

// directDecodeBit decodes a bit with probability 1/2. The return value b will
// contain the bit at the least-significant position. All other bits will be
// zero.
func (d *rangeDecoder) directDecodeBit() (b uint32, err error) {
	nrange := d.nrange >> 1
	d.code -= nrange
	t := 0 - (d.code >> 31)
	d.code += nrange & t
	b = (t + 1) & 1

	// d.code will stay less then d.nrange

	// normalize
	// assume d.code < d.nrange
	if nrange >= (1 << 24) {
		d.nrange = nrange
		return b, nil
	}
	d.nrange = nrange << 8
	// d.code < d.nrange will be maintained
	return b, d.updateCode()
}

// decodeBit decodes a single bit. The bit will be returned at the
// least-significant position. All other bits will be zero. The probability
// value will be updated.
func (d *rangeDecoder) decodeBit(p *prob) (b uint32, err error) {
	// assume d.code < d.nrange
	nrange := d.nrange
	bound := p.bound(nrange)
	if d.code < bound {
		*p = incProb(*p)
		b = 0
		nrange = bound
	} else {
		*p = decProb(*p)
		b = 1
		d.code -= bound
		nrange -= bound
	}
	// normalize
	// assume d.code < d.nrange
	if nrange >= (1 << 24) {
		d.nrange = nrange
		return b, nil
	}
	d.nrange = nrange << 8
	// d.code < d.nrange will be maintained
	return b, d.updateCode()
}

// updateCode reads a new byte into the code.
func (d *rangeDecoder) updateCode() error {
	b, err := d.br.ReadByte()
	if err != nil {
		return err
	}
	d.code = (d.code << 8) | uint32(b)
	return nil
}
