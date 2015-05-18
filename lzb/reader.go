package lzb

import (
	"errors"
	"fmt"
	"io"
)

// Reader provides a basic LZMA reader. It doesn't support any header
// but allows a reset keeping the state.
type Reader struct {
	State  *State
	rd     *rangeDecoder
	buf    *buffer
	head   int64
	eof    bool
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
	dict, err := newSyncDict(buf, params.DictSize)
	if err != nil {
		return nil, err
	}
	state := NewState(params.Properties, dict)
	return NewReaderState(rr, state)
}

func NewReaderState(rr io.Reader, state *State) (r *Reader, err error) {
	if _, ok := state.dict.(*syncDict); !ok {
		return nil, errors.New(
			"state must support a reader (no syncDict)")
	}
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
	r.buf.writeLimit = off + int64(r.buf.capacity())
	return off, nil
}

func (r *Reader) readBuffer(p []byte) (n int, err error) {
	n, err = r.buf.ReadAt(p, r.head)
	if _, serr := r.seek(int64(n), 1); serr != nil {
		panic(fmt.Errorf("r.seek(%d, 1) error %s", int64(n), serr))
	}
	return
}

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
		if r.closed {
			r.eof = true
			return n, io.EOF
		}
		p = p[k:]
		if err = r.fillBuffer(); err != nil {
			return n, err
		}
	}
}

// Errors produced by readOp and fillBuffer
var (
	eos              = errors.New("end of stream")
	errUnexpectedEOS = errors.New("data after end of stream")
	errUnexpectedEOF = errors.New("unexpected end of compressed stream")
)

// decodeLiteral reads a literal.
func (r *Reader) decodeLiteral() (op operation, err error) {
	litState := r.State.litState()

	match := r.State.dict.byteAt(int64(r.State.rep[0]) + 1)
	s, err := r.State.litCodec.Decode(r.rd, r.State.state, match, litState)
	if err != nil {
		return nil, err
	}
	return lit{s}, nil
}

// readOp decodes the next operation from the compressed stream. It returns the
// operation. If an explicit end of stream marker is identified the eos error is
// returned.
func (r *Reader) readOp() (op operation, err error) {
	// Value of the end of stream (EOS) marker
	const eosDist = 1<<32 - 1

	state, state2, posState := r.State.states()

	b, err := r.State.isMatch[state2].Decode(r.rd)
	if err != nil {
		return nil, err
	}
	if b == 0 {
		// literal
		op, err := r.decodeLiteral()
		if err != nil {
			return nil, err
		}
		r.State.updateStateLiteral()
		return op, nil
	}
	b, err = r.State.isRep[state].Decode(r.rd)
	if err != nil {
		return nil, err
	}
	if b == 0 {
		// simple match
		r.State.rep[3], r.State.rep[2], r.State.rep[1] = r.State.rep[2], r.State.rep[1], r.State.rep[0]

		r.State.updateStateMatch()
		// The length decoder returns the length offset.
		n, err := r.State.lenCodec.Decode(r.rd, posState)
		if err != nil {
			return nil, err
		}
		// The dist decoder returns the distance offset. The actual
		// distance is 1 higher.
		r.State.rep[0], err = r.State.distCodec.Decode(r.rd, n)
		if err != nil {
			return nil, err
		}
		if r.State.rep[0] == eosDist {
			if !r.rd.possiblyAtEnd() {
				return nil, errUnexpectedEOS
			}
			return nil, eos
		}
		op = match{n: int(n) + MinLength,
			distance: int64(r.State.rep[0]) + minDistance}
		return op, nil
	}
	b, err = r.State.isRepG0[state].Decode(r.rd)
	if err != nil {
		return nil, err
	}
	dist := r.State.rep[0]
	if b == 0 {
		// rep match 0
		b, err = r.State.isRepG0Long[state2].Decode(r.rd)
		if err != nil {
			return nil, err
		}
		if b == 0 {
			r.State.updateStateShortRep()
			op = match{n: 1, distance: int64(dist) + minDistance}
			return op, nil
		}
	} else {
		b, err = r.State.isRepG1[state].Decode(r.rd)
		if err != nil {
			return nil, err
		}
		if b == 0 {
			dist = r.State.rep[1]
		} else {
			b, err = r.State.isRepG2[state].Decode(r.rd)
			if err != nil {
				return nil, err
			}
			if b == 0 {
				dist = r.State.rep[2]
			} else {
				dist = r.State.rep[3]
				r.State.rep[3] = r.State.rep[2]
			}
			r.State.rep[2] = r.State.rep[1]
		}
		r.State.rep[1] = r.State.rep[0]
		r.State.rep[0] = dist
	}
	n, err := r.State.repLenCodec.Decode(r.rd, posState)
	if err != nil {
		return nil, err
	}
	r.State.updateStateRep()
	op = match{n: int(n) + MinLength, distance: int64(dist) + minDistance}
	return op, nil
}

func (r *Reader) fillBuffer() error {
	if r.closed {
		return nil
	}
	d := r.State.dict.(*syncDict)
	for r.buf.top+MaxLength <= r.buf.writeLimit {
		op, err := r.readOp()
		if err != nil {
			switch err {
			case eos:
				r.closed = true
				return nil
			case io.EOF:
				r.closed = true
				return errUnexpectedEOF
			default:
				return err
			}
		}
		if err = op.Apply(d); err != nil {
			return err
		}
	}
	return nil
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
