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

func newReadBuffer(capacity int64, dictsize int64) (r *readBuffer, err error) {
	b, err := newBuffer(capacity)
	if err != nil {
		return nil, err
	}
	d, err := newDict(b, 0, dictsize)
	if err != nil {
		return nil, err
	}
	r = &readBuffer{buffer: b, dict: d, head: 0}
	return r, nil
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

func (r *readBuffer) Write(p []byte) (n int, err error) {
	n, err = r.Write(p)
	_, serr := r.dict.Seek(int64(n), 1)
	if err == nil {
		err = serr
	}
	return
}

func (r *readBuffer) WriteByte(c byte) error {
	err := r.WriteByte(c)
	if err != nil {
		return err
	}
	_, err = r.dict.Seek(1, 1)
	return err
}
