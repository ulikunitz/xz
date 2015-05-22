package lzb

import (
	"errors"
)

// dict represents a dictionary. A dictionary is described by its size
// and the buffer storing the actual bytes. Multiple dictionaries
// at different positions may share the same buffer.
type dict struct {
	buf *buffer
	// position of the dictionary in the byte stream covered by
	// buffer
	head int64
	// size of the dictionary
	size int64
}

// newDict creates a new dictionary. The head must be a valid offset
// mapping into the buffer. The dictionary size cannot exceed the buffer
// size.
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

// Returns the byte at the given distance. The distance must be positive
// and cannot exceed the size of the dictionary. If the position at the
// distance is not covered by the backing buffer zero will be returned.
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

// errWhence marks an invalid whence value.
var errWhence = errors.New("unsupported whence value")

// seek sets the dictionary head to a specific value. The whence value
// 0 takes the offset as absolute value. The whence value 1 interprets
// the offset relative to the current position. An offset 0 in that case
// will cause seek to provide the current head without changing its
// value. The whence value 2 applies the offset relative to the end of
// the buffer mapping the byte stream.
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

// buffer returns the backing buffer.
func (d *dict) buffer() *buffer {
	return d.buf
}

// start returns the start of the dictionary.
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

// newSyncDict creates a new synchronized dictionary.
func newSyncDict(buf *buffer, size int64) (d *syncDict, err error) {
	var t *dict
	t, err = newDict(buf, buf.top, size)
	if err != nil {
		return nil, err
	}
	return &syncDict{dict: *t}, nil
}

// writeRep writes a repetition to the top of the buffer and keeps the
// head of the dictionary synchronous with the buffer.
func (d *syncDict) writeRep(dist int64, n int) (written int, err error) {
	off := d.head - dist
	written, err = d.buf.writeRepAt(n, off)
	d.head = d.buf.top
	return
}

// writeByte writes a single byte to the top of the buffer and keeps the
// dictionary head equal with the buffer top.
func (d *syncDict) writeByte(c byte) error {
	err := d.buf.WriteByte(c)
	d.head = d.buf.top
	return err
}

// hashDict combines the dictionary with a hash table of four-byte
// sequences of the byte stream covered by the buffer. This type will
// support the lzb.Writer.
type hashDict struct {
	dict
	t4 hashTable
}

// newHashDict creates a new hashDict instance.
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

// move advances the head n bytes forward and record the new data in the
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

// sync synchronizes the write limit of the backing buffer with the
// current dictionary head.
func (d *hashDict) sync() {
	d.buf.writeLimit = d.start() + int64(d.buf.capacity())
}
