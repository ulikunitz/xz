package lzma

import "io"

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

// DirectEncodeBit encodes the least-significant bit of b with probability 1/2.
func (e *rangeEncoder) DirectEncodeBit(b uint32) error {
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

// EncodeBit encodes the least significant bit of b. The p value will be
// updated by the function depending on the bit encoded.
func (e *rangeEncoder) EncodeBit(b uint32, p *prob) error {
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
	const top = 1 << 24
	if nrange >= top {
		e.nrange = nrange
		return nil
	}
	e.nrange = nrange << 8
	return e.shiftLow()
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
