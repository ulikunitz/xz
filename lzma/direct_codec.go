package lzma

import "fmt"

type directEncoder struct {
	bits byte
}

func newDirectEncoder(bits byte) *directEncoder {
	if !(1 <= bits && bits <= 32) {
		panic(fmt.Errorf("bits=%d out of range", bits))
	}
	return &directEncoder{bits}
}

func (de *directEncoder) Encode(v uint32, e *rangeEncoder) error {
	for i := int(de.bits) - 1; i >= 0; i-- {
		if err := e.DirectEncodeBit(v >> uint(i)); err != nil {
			return err
		}
	}
	return nil
}

type directDecoder struct {
	bits byte
}

func newDirectDecoder(bits byte) *directDecoder {
	if !(1 <= bits && bits <= 32) {
		panic(fmt.Errorf("bits=%d out of range", bits))
	}
	return &directDecoder{bits}
}

func (dd *directDecoder) Decode(d *rangeDecoder) (v uint32, err error) {
	for i := int(dd.bits) - 1; i >= 0; i-- {
		x, err := d.DirectDecodeBit()
		if err != nil {
			return 0, err
		}
		v = (v << 1) | x
	}
	return v, nil
}
