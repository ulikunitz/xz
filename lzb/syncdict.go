package lzb

import "errors"

type syncDict struct {
	buf  *buffer
	size int64
}

func (sd *syncDict) offset() int64 {
	return sd.buf.top
}

func (sd *syncDict) byteAt(dist int64) byte {
	if !(0 < dist && dist <= sd.size) {
		panic("dist out of range")
	}
	off := sd.buf.top - dist
	if off < sd.buf.bottom {
		return 0
	}
	return sd.buf.data[sd.buf.index(off)]
}

func (sd *syncDict) reset() {
	sd.buf.reset()
}

// writeRep writes a repetition to the top of the buffer and keeps the
// head of the dictionary synchronous with the buffer.
func (sd *syncDict) writeRep(dist int64, n int) (written int, err error) {
	if !(0 < dist && dist <= sd.size) {
		panic("dist out of range")
	}
	off := sd.buf.top - dist
	written, err = sd.buf.writeRepAt(n, off)
	return
}

func (sd *syncDict) WriteByte(c byte) error {
	return sd.buf.WriteByte(c)
}

func newSyncDict(buf *buffer, size int64) (sd *syncDict, err error) {
	if !(MinDictSize <= size && size <= int64(buf.capacity())) {
		return nil, errors.New("size out of range")
	}
	sd = &syncDict{buf: buf, size: size}
	return sd, nil
}
