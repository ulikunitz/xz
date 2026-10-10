package nlzma

import (
	"errors"
	"io"
)

type rangeEncoder interface {
	directEncodeBit(b uint32)
	encodeBit(p prob, b uint32) prob
	Close() error
}

type rEncoder struct {
	p        []byte
	low      uint64
	cacheLen int
	nrange   uint32
	cache    byte
}

func (e *rEncoder) init(p []byte) {
	*e = rEncoder{
		p:        p[:0],
		nrange:   1<<32 - 1,
		cacheLen: 1,
	}
}

func (e *rEncoder) Close() error {
	for range 5 {
		e.shiftLow()
	}
	return nil
}

func (e *rEncoder) directEncodeBit(b uint32) {
	e.nrange >>= 1
	e.low += uint64(e.nrange) & (0 - (uint64(b) & 1))

	if e.nrange >= (1 << 24) {
		return
	}
	e.nrange <<= 8
	e.shiftLow()
}

func (e *rEncoder) encodeBit(p prob, b uint32) prob {
	nrange := e.nrange
	bound := p.bound(nrange)
	if b&1 == 0 {
		nrange = bound
		p = incProb(p)
	} else {
		e.low += uint64(bound)
		nrange -= bound
		p = decProb(p)
	}

	// normalize
	if nrange >= (1 << 24) {
		e.nrange = nrange
		return p
	}
	e.nrange = nrange << 8
	e.shiftLow()
	return p
}

func (e *rEncoder) shiftLow() {
	if uint32(e.low) < 0xff000000 || uint32(e.low>>32) != 0 {
		tmp := e.cache
		for {
			e.p = append(e.p, tmp+byte(e.low>>32))
			tmp = 0xff
			e.cacheLen--
			if e.cacheLen <= 0 {
				// TODO: remove
				if e.cacheLen < 0 {
					panic("cacheLen should not be negative")
				}
				break
			}
		}
		e.cache = byte(uint32(e.low) >> 24)
	}
	e.cacheLen++
	e.low = uint64(uint32(e.low) << 8)
}

type rDecoder struct {
	p      []byte
	nrange uint32
	code   uint32
}

func (d *rDecoder) init(p []byte) error {
	*d = rDecoder{
		p:      p,
		nrange: 1<<32 - 1,
	}

	if len(p) == 0 {
		return io.ErrUnexpectedEOF
	}
	b := p[0]
	d.p = p[1:]
	if b != 0 {
		return errors.New(
			"lzma: first byte of LZMA stream must be zero")
	}
	for range 4 {
		if len(d.p) == 0 {
			return io.ErrUnexpectedEOF
		}
		d.updateCode()
	}
	if d.code >= d.nrange {
		return errors.New("lzma: d.code >= d.nrange")
	}
	return nil
}

func (d *rDecoder) possiblyAtEnd() bool {
	return d.code == 0
}

func (d *rDecoder) updateCode() {
	if len(d.p) == 0 {
		return
	}
	d.code = (d.code << 8) | uint32(d.p[0])
	d.p = d.p[1:]
}

// directDecodeBit decodes a bit with probability 1/2. The return value b will
// contain the bit at the least-significant position. All other bits will be
// zero.
func (d *rDecoder) directDecodeBit() uint32 {
	nrange := d.nrange >> 1
	d.code -= nrange
	t := 0 - (d.code >> 31)
	d.code += nrange & t
	b := (t + 1) & 1

	// d.code will stay less then d.nrange

	// normalize
	// assume d.code < d.nrange
	if nrange >= (1 << 24) {
		d.nrange = nrange
		return b
	}
	d.nrange = nrange << 8
	// d.code < d.nrange will be maintained
	d.updateCode()
	return b
}

// decodeBit decodes a single bit. The bit will be returned at the
// least-significant position. All other bits will be zero. The probability
// value will be updated.
func (d *rDecoder) decodeBit(p prob) (b uint32, r prob) {
	// assume d.code < d.nrange
	nrange := d.nrange
	bound := p.bound(nrange)
	if d.code < bound {
		p = incProb(p)
		b = 0
		nrange = bound
	} else {
		p = decProb(p)
		b = 1
		d.code -= bound
		nrange -= bound
	}
	// normalize
	// assume d.code < d.nrange
	if nrange >= (1 << 24) {
		d.nrange = nrange
		return b, p
	}
	d.nrange = nrange << 8
	// d.code < d.nrange will be maintained
	d.updateCode()
	return b, p
}
