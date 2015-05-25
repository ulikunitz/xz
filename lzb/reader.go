package lzb

import (
	"errors"
	"fmt"
	"io"
)

type Reader struct {
	*opReader
	head    int64
	limited bool
	limit   int64
	eof     bool
}

var errUnexpectedEOS = errors.New("unexpected eos")

// seek moves the reader head using the classic whence mechanism.
func (r *Reader) seek(offset int64, whence int) (off int64, err error) {
	switch whence {
	case 0:
		off = offset
	case 1:
		if offset == 0 {
			return r.head, nil
		}
		off = r.head + offset
	case 2:
		off = r.buf.top + offset
	default:
		return r.head, errWhence
	}
	if !(r.buf.bottom <= off && off <= r.buf.top) {
		return r.head, errOffset
	}
	limit := off + int64(r.buf.capacity())
	if r.limited && limit > r.limit {
		limit = r.limit
	}
	if limit < r.buf.top {
		return r.head, errors.New("write limit out of range")
	}
	r.head, r.buf.writeLimit = off, limit
	return off, nil
}

// readBuffer reads data from the buffer into the p slice.
func (r *Reader) readBuffer(p []byte) (n int, err error) {
	n, err = r.buf.ReadAt(p, r.head)
	if _, serr := r.seek(int64(n), 1); serr != nil {
		panic(fmt.Errorf("r.seek(%d, 1) error %s", int64(n), serr))
	}
	if r.closed && r.head == r.buf.top {
		r.eof = true
		err = io.EOF
	}
	return
}

// Read reads uncompressed data from the raw LZMA data stream.
func (r *Reader) Read(p []byte) (n int, err error) {
	if r.eof {
		return 0, io.EOF
	}
	if len(p) == 0 {
		return 0, nil
	}
	for {
		var k int
		k, err = r.readBuffer(p)
		n += k
		if err != nil {
			return
		}
		if k >= len(p) {
			return
		}
		p = p[k:]
		err = r.fillBuffer()
		if err != nil {
			if err == eos {
				if r.limited && r.limit != r.buf.top {
					return n, errUnexpectedEOS
				}
			} else {
				return n, err
			}
		}
		if r.limited && r.limit == r.buf.top {
			err = r.opReader.close()
			if err != nil {
				return n, err
			}
		}
	}
}

func NewReader(lzma io.Reader, p Parameters) (r *Reader, err error) {
	if err = verifyParameters(&p); err != nil {
		return nil, err
	}
	buf, err := newBuffer(p.BufferSize)
	if err != nil {
		return
	}
	dict, err := newSyncDict(buf, p.DictSize)
	if err != nil {
		return
	}
	state := NewState(p.Properties(), dict)
	or, err := newOpReader(lzma, state)
	if err != nil {
		return
	}
	r = &Reader{opReader: or, head: buf.bottom}
	if p.SizeInHeader {
		r.limited = true
		r.limit = r.head + p.Size
		if r.limit < r.head {
			return nil, errors.New("limit out of range")
		}
	}
	return r, nil
}

func (r *Reader) Restart(raw io.Reader) {
	panic("TODO")
}

func (r *Reader) ResetState() {
	panic("TODO")
}

func (r *Reader) ResetProperties(p Properties) {
	panic("TODO")
}

func (r *Reader) ResetDictionary(p Properties) {
	panic("TODO")
}
