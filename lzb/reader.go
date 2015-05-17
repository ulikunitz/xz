package lzb

import "io"

// Reader provides a basic LZMA reader. It doesn't support any header
// but allows a reset keeping the state.
type Reader struct {
	State  *State
	rd     *rangeDecoder
	buf    *buffer
	head   int64
	closed bool
}

type Params struct {
	Properties Properties
	BufferSize int64
	DictSize   int64
}

func NewReader(rr io.Reader, params Params) (r *Reader, err error) {
	buf, err := newBuffer(params.BufferSize)
	if err != nil {
		return nil, err
	}
	dict, err := newDict(buf, 0, params.DictSize)
	if err != nil {
		return nil, err
	}
	state := NewState(params.Properties, dict)
	return NewReaderState(rr, state)
}

func NewReaderState(rr io.Reader, state *State) (r *Reader, err error) {
	r = &Reader{State: state, buf: state.dict.buffer()}
	r.rd, err = newRangeDecoder(rr)
	if err != nil {
		return nil, err
	}
	if _, err = r.seek(r.buf.bottom, 0); err != nil {
		return nil, err
	}
	return r, nil
}

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
	r.head = off
	return off, nil
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
