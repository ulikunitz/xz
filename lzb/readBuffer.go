package lzb

type readBuffer struct {
	*buffer
	dict *dict
	head int64
}

func (r *readBuffer) Seek(offset int64, whence int) (off int64, err error) {
	switch whence {
	case 0:
		off = offset
	case 1:
		off = r.head + offset
	case 2:
		off = r.top + offset
	default:
		return r.head, errWhence
	}
	if !(r.bottom <= off && off <= r.top) {
		return r.head, errOffset
	}
	r.head = off
	r.writeLimit = off + int64(r.capacity())
	return
}

func newReadBuffer(d *dict) *readBuffer {
	r := &readBuffer{buffer: d.buf, dict: d}
	if _, err := r.Seek(r.bottom, 0); err != nil {
		panic(err)
	}
	return r
}

func (r *readBuffer) Read(p []byte) (n int, err error) {
	n, err = r.ReadAt(p, r.head)
	if err != nil {
		return 0, err
	}
	_, err = r.Seek(int64(n), 1)
	if err != nil {
		return 0, err
	}
	return
}
