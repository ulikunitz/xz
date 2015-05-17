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
		return nil, errOffset
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

var errWhence = errors.New("unsupported whence value")

func (d *dict) seek(offset int64, whence int) (off int64, err error) {
	switch whence {
	case 0:
		off = offset
	case 1:
		if offset == 0 {
			return d.head, nil
		}
		off = d.head + offset
	case 2:
		off = d.buf.top + offset
	default:
		return d.head, errWhence
	}
	if !(d.buf.bottom <= off && off <= d.buf.top) {
		return d.head, errOffset
	}
	d.head = off
	return
}

func (d *dict) buffer() *buffer {
	return d.buf
}
