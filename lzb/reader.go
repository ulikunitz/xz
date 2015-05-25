package lzb

import (
	"errors"
	"fmt"
	"io"
)

var errUnexpectedEOS = errors.New("unexpected eos")

type Reader struct {
	*opReader
	eof     bool
	buf     *buffer
	head    int64
	limited bool
	limit   int64
}

func (r *Reader) move(n int64) (off int64, err error) {
	off = r.head + n
	if !(r.buf.bottom <= off && off <= r.buf.top) {
		return r.head, errors.New("new offset out of range")
	}
	limit := off + int64(r.buf.capacity())
	if r.limited && limit > r.limit {
		limit = r.limit
	}
	if limit < r.buf.top {
		return r.head, errors.New("limit out of range")
	}
	r.head = off
	r.buf.writeLimit = limit
	return off, nil
}

// readBuffer reads data from the buffer into the p slice.
func (r *Reader) readBuffer(p []byte) (n int, err error) {
	n, err = r.buf.ReadAt(p, r.head)
	_, merr := r.move(int64(n))
	if merr != nil {
		panic(fmt.Errorf("r.move(%d) error %s", int64(n), merr))
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
				if r.limited && r.head != r.limit {
					return n, errUnexpectedEOS
				}
			} else {
				return n, err
			}
		}
		if r.limited && r.head == r.limit {
			err = r.opReader.close()
			if err != nil {
				return n, err
			}
		}
	}
}

func (r *Reader) setSize(size int64) error {
	limit := r.head + size
	if r.buf.top > limit {
		return errors.New("limit out of range")
	}
	r.limited = true
	r.limit = limit
	if _, err := r.move(0); err != nil {
		panic(err)
	}
	return nil
}

func NewReader(lzma io.Reader, p Parameters) (r *Reader, err error) {
	if err = p.Verify(); err != nil {
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
	r = &Reader{opReader: or, buf: buf, head: buf.bottom}
	if p.SizeInHeader {
		if err = r.setSize(p.Size); err != nil {
			return nil, err
		}
	} else {
		if _, err = r.move(0); err != nil {
			panic(err)
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
