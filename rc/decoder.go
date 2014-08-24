package rc

import (
	"errors"
	"io"
)

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

func (d *Decoder) DecodeDirect() (b Bit, err error) {
	d.range_ >>= 1
	d.code -= d.range_
	t := 0 - (d.code >> 31)
	d.code += d.range_ & t

	// d.code will stay less then d.range_

	if err = d.normalize(); err != nil {
		return 0, err
	}
	return Bit((t + 1) & 1), nil
}

func (d *Decoder) Decode(p *Prob) (b Bit, err error) {
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
