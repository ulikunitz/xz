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

func (d *dict) start() int64 {
	start := d.head - d.size
	if start < d.buf.bottom {
		start = d.buf.bottom
	}
	return start
}

// syncDict synchronizes buf.top with dict.head.
type syncDict struct {
	dict
}

func newSyncDict(buf *buffer, size int64) (d *syncDict, err error) {
	var t *dict
	t, err = newDict(buf, buf.top, size)
	if err != nil {
		return nil, err
	}
	return &syncDict{dict: *t}, nil
}

func (d *syncDict) writeRep(dist int64, n int) (written int, err error) {
	off := d.head - dist
	written, err = d.buf.writeRepAt(n, off)
	d.head = d.buf.top
	return
}

func (d *syncDict) writeByte(c byte) error {
	err := d.buf.WriteByte(c)
	d.head = d.buf.top
	return err
}

type hashDict struct {
	dict
	t4 hashTable
}

func newHashDict(buf *buffer, size int64) (d *hashDict, err error) {
	t4, err := newHashTable(size, 4)
	if err != nil {
		return nil, err
	}
	var t *dict
	t, err = newDict(buf, buf.top, size)
	if err != nil {
		return nil, err
	}
	return &hashDict{dict: *t, t4: *t4}, nil
}

// Move moves the head n bytes forward and record the new data in the
// hash table.
func (d *hashDict) move(n int) (moved int, err error) {
	if n < 0 {
		return 0, errors.New("argument n must be non-negative")
	}
	if !(d.buf.bottom <= d.head && d.head <= d.buf.top) {
		panic("head out of range")
	}
	off := d.head + int64(n)
	if off > d.buf.top {
		off = d.buf.top
	}
	moved, err = d.buf.writeRangeTo(d.head, off, &d.t4)
	d.head += int64(moved)
	return
}

func (d *hashDict) sync() {
	d.buf.writeLimit = d.start() + int64(d.buf.capacity())
}
