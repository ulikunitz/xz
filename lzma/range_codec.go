package lzma

import (
	"io"

	"github.com/uli-go/xz/xlog"
)

// newRangeEncoder creates a new range encoder.
func newRangeEncoder(w io.ByteWriter) *rangeEncoder {
	return &rangeEncoder{w: w, range_: 0xffffffff, cacheSize: 1}
}

// DirectEncodeBit encodes the least-significant bit of b with probability 1/2.
func (e *rangeEncoder) DirectEncodeBit(b uint32) error {
	e.range_ >>= 1
	e.low += uint64(e.range_) & (0 - (uint64(b) & 1))
	return e.normalize()
}

// EncodeBit encodes the least significant bit of b. The p value will be
// updated by the function depending on the bit encoded.
func (e *rangeEncoder) EncodeBit(b uint32, p *prob) error {
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

// Flush writes a complete copy of the low value.
func (e *rangeEncoder) Flush() error {
	for i := 0; i < 5; i++ {
		if err := e.shiftLow(); err != nil {
			return err
		}
	}
	return nil
}

// newRangeDecoder initializes a range decoder. It reads five bytes from the
// reader and therefore may return an error.
func newRangeDecoder(r io.ByteReader) (d *rangeDecoder, err error) {
	d = &rangeDecoder{r: r}
	err = d.init()
	return
}

// possiblyAtEnd checks whether the decoder may be at the end of the stream.
func (d *rangeDecoder) possiblyAtEnd() bool {
	return d.code == 0
}

var bitCounter int

// DirectDecodeBit decodes a bit with probability 1/2. The return value b will
// contain the bit at the least-significant position. All other bits will be
// zero.
func (d *rangeDecoder) DirectDecodeBit() (b uint32, err error) {
	bitCounter++
	xlog.Printf(Debug, "D %3d 0x%08x:0x%08x\n", bitCounter, d.range_,
		d.code)
	d.range_ >>= 1
	d.code -= d.range_
	t := 0 - (d.code >> 31)
	d.code += d.range_ & t

	// d.code will stay less then d.range_

	if err = d.normalize(); err != nil {
		return 0, err
	}

	b = (t + 1) & 1

	xlog.Printf(Debug, "O %3d %d\n", bitCounter, b)
	return b, nil
}

// decodeBit decodes a single bit. The bit will be returned at the
// least-significant position. All other bits will be zero. The probability
// value will be updated.
func (d *rangeDecoder) DecodeBit(p *prob) (b uint32, err error) {
	bitCounter++
	xlog.Printf(Debug, "B %3d 0x%08x:0x%08x 0x%03x\n", bitCounter,
		d.range_, d.code, *p)
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

	xlog.Printf(Debug, "O %3d %d\n", bitCounter, b)

	return b, nil
}

// rangeEncoder implements range encoding of single bits. The low value can
// overflow therefore we need uint64. The cache value is used to handle
// overflows.
type rangeEncoder struct {
	w         io.ByteWriter
	range_    uint32
	low       uint64
	cacheSize int64
	cache     byte
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
					return newError("negative e.cacheSize")
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

// rangeDecoder decodes single bits of the range encoding stream.
type rangeDecoder struct {
	r      io.ByteReader
	range_ uint32
	code   uint32
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
		return newError("first byte not zero")
	}

	for i := 0; i < 4; i++ {
		if err = d.updateCode(); err != nil {
			return err
		}
	}

	if d.code >= d.range_ {
		return newError("newRangeDecoder: d.code >= d.range_")
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
