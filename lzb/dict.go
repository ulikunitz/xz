package lzb

import "errors"

type dict struct {
	buf  *buffer
	top  int64
	size int64
}

func makeDict(b *buffer, top int64, size int64) (d *dict, err error) {
	switch {
	case size <= 0:
		return nil, errors.New("size must be positive")
	case size > int64(b.capacity()):
		return nil, errors.New("size exceeds buffer capacity")
	case !(b.bottom <= top && top <= b.top):
		return nil, errors.New("top offset out of range")
	}
	return &dict{buf: b, top: top, size: size}, nil
}

func (d *dict) byteAt(dist int64) byte {
	if dist <= 0 {
		panic("dist must be positve")
	}
	off := d.top - dist
	if off < d.buf.bottom {
		return 0
	}
	return d.buf.data[d.buf.index(off)]
}

func (d *dict) move(n int) error {
	off := d.top + int64(n)
	if !(d.buf.bottom <= off && off <= d.buf.top) {
		return errors.New("move outside buffer")
	}
	d.top = off
	return nil
}
