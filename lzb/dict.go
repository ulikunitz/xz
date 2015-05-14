package lzb

import "errors"

type dict struct {
	buf  *buffer
	head int64
	size int64
}

func newDict(b *buffer, head int64, size int64) (d *dict, err error) {
	switch {
	case size <= 0:
		return nil, errors.New("size must be positive")
	case size > int64(b.capacity()):
		return nil, errors.New("size exceeds buffer capacity")
	case !(b.bottom <= head && head <= b.top):
		return nil, errors.New("head offset out of range")
	}
	return &dict{buf: b, head: head, size: size}, nil
}

func (d *dict) byteAt(dist int64) byte {
	if !(0 < dist && dist <= d.size) {
		panic("dist out of range")
	}
	off := d.head - dist
	if off < d.buf.bottom {
		return 0
	}
	return d.buf.data[d.buf.index(off)]
}

var errMove = errors.New("move outside buffer")

func (d *dict) move(n int) error {
	off := d.head + int64(n)
	if !(d.buf.bottom <= off && off <= d.buf.top) {
		return errMove
	}
	d.head = off
	return nil
}
