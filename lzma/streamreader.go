package lzma

import (
	"errors"
	"io"
)

// Errors produced by readOp and fillBuffer
var (
	eos              = errors.New("end of stream")
	errClosed        = errors.New("stream is closed")
	errDataAfterEOS  = errors.New("data after end of stream")
	errUnexpectedEOS = errors.New("unexpected eos")
)

type Reader struct {
	Params    Parameters
	state     *State
	rd        *rangeDecoder
	pendingOp operation
	buf       *buffer
	head      int64
	limited   bool
	// limit marks the expected size of the decompressed byte stream
	limit  int64
	closed bool
}

func NewStreamReader(lzma io.Reader, p Parameters) (r *Reader, err error) {
	if err = p.Verify(); err != nil {
		return nil, err
	}
	buf, err := newBuffer(p.DictSize + p.ExtraBufSize)
	if err != nil {
		return
	}
	dict, err := newSyncDict(buf, p.DictSize)
	if err != nil {
		return
	}
	state := NewState(p.Properties(), dict)
	rd, err := newRangeDecoder(lzma)
	if err != nil {
		return
	}
	r = &Reader{Params: p, state: state, rd: rd, buf: buf, head: buf.bottom}
	if p.SizeInHeader {
		if err = r.setSize(p.Size); err != nil {
			return nil, err
		}
	} else {
		r.move(0)
	}
	return r, nil
}

// decodeLiteral reads a literal.
func (r *Reader) decodeLiteral() (op operation, err error) {
	litState := r.state.litState()
	match := r.state.dict.byteAt(int64(r.state.rep[0]) + 1)
	s, err := r.state.litCodec.Decode(r.rd, r.state.state, match, litState)
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

	state, state2, posState := r.state.states()

	b, err := r.state.isMatch[state2].Decode(r.rd)
	if err != nil {
		return nil, err
	}
	if b == 0 {
		// literal
		op, err := r.decodeLiteral()
		if err != nil {
			return nil, err
		}
		r.state.updateStateLiteral()
		return op, nil
	}
	b, err = r.state.isRep[state].Decode(r.rd)
	if err != nil {
		return nil, err
	}
	if b == 0 {
		// simple match
		r.state.rep[3], r.state.rep[2], r.state.rep[1] = r.state.rep[2], r.state.rep[1], r.state.rep[0]

		r.state.updateStateMatch()
		// The length decoder returns the length offset.
		n, err := r.state.lenCodec.Decode(r.rd, posState)
		if err != nil {
			return nil, err
		}
		// The dist decoder returns the distance offset. The actual
		// distance is 1 higher.
		r.state.rep[0], err = r.state.distCodec.Decode(r.rd, n)
		if err != nil {
			return nil, err
		}
		if r.state.rep[0] == eosDist {
			return nil, eos
		}
		op = match{n: int(n) + MinLength,
			distance: int64(r.state.rep[0]) + minDistance}
		return op, nil
	}
	b, err = r.state.isRepG0[state].Decode(r.rd)
	if err != nil {
		return nil, err
	}
	dist := r.state.rep[0]
	if b == 0 {
		// rep match 0
		b, err = r.state.isRepG0Long[state2].Decode(r.rd)
		if err != nil {
			return nil, err
		}
		if b == 0 {
			r.state.updateStateShortRep()
			op = match{n: 1, distance: int64(dist) + minDistance}
			return op, nil
		}
	} else {
		b, err = r.state.isRepG1[state].Decode(r.rd)
		if err != nil {
			return nil, err
		}
		if b == 0 {
			dist = r.state.rep[1]
		} else {
			b, err = r.state.isRepG2[state].Decode(r.rd)
			if err != nil {
				return nil, err
			}
			if b == 0 {
				dist = r.state.rep[2]
			} else {
				dist = r.state.rep[3]
				r.state.rep[3] = r.state.rep[2]
			}
			r.state.rep[2] = r.state.rep[1]
		}
		r.state.rep[1] = r.state.rep[0]
		r.state.rep[0] = dist
	}
	n, err := r.state.repLenCodec.Decode(r.rd, posState)
	if err != nil {
		return nil, err
	}
	r.state.updateStateRep()
	op = match{n: int(n) + MinLength, distance: int64(dist) + minDistance}
	return op, nil
}

func (r *Reader) close() error {
	if r.closed {
		return errClosed
	}
	if r.pendingOp != nil {
		return errDataAfterEOS
	}
	if !r.rd.possiblyAtEnd() {
		_, err := r.readOp()
		if err != eos {
			if err != nil {
				return err
			}
			return errDataAfterEOS
		}
		if !r.rd.possiblyAtEnd() {
			return errDataAfterEOS
		}
	}
	r.closed = true
	return nil
}

func (r *Reader) outsideLimits(op operation) (outside bool, err error) {
	off := r.buf.top + int64(op.Len())
	if off > r.buf.writeLimit {
		if r.limited && off > r.limit {
			return true, errLimit
		}
		return true, nil
	}
	return false, nil
}

// fillBuffer fills the buffer with data read from the LZMA stream.
func (r *Reader) fillBuffer() error {
	if r.closed {
		return nil
	}
	d := r.state.dict.(*syncDict)
	if r.pendingOp != nil {
		op := r.pendingOp
		if outside, err := r.outsideLimits(op); outside || err != nil {
			return err
		}
		if err := op.Apply(d); err != nil {
			return err
		}
		r.pendingOp = nil
	}
	for r.buf.top < r.buf.writeLimit {
		op, err := r.readOp()
		if err != nil {
			switch err {
			case eos:
				r.closed = true
				if !r.rd.possiblyAtEnd() {
					return errDataAfterEOS
				}
				return eos
			case io.EOF:
				r.closed = true
				return io.ErrUnexpectedEOF
			default:
				return err
			}
		}
		if outside, err := r.outsideLimits(op); outside || err != nil {
			r.pendingOp = op
			return err
		}
		if err = op.Apply(d); err != nil {
			return err
		}
	}
	return nil
}

func (r *Reader) move(n int) {
	off := r.head + int64(n)
	if !(r.buf.bottom <= off && off <= r.buf.top) {
		panic("new offset out of range")
	}
	limit := off + int64(r.buf.capacity())
	if r.limited && limit > r.limit {
		limit = r.limit
	}
	if limit < r.buf.top {
		panic("limit out of range")
	}
	r.head = off
	r.buf.writeLimit = limit
}

func (r *Reader) eof() bool {
	return r.closed && r.head == r.buf.top
}

// readBuffer reads data from the buffer into the p slice.
func (r *Reader) readBuffer(p []byte) (n int, err error) {
	n, err = r.buf.ReadAt(p, r.head)
	r.move(n)
	if r.eof() {
		err = io.EOF
	}
	return
}

// Read reads uncompressed data from the raw LZMA data stream.
func (r *Reader) Read(p []byte) (n int, err error) {
	if r.eof() {
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
				if r.limited && r.buf.top != r.limit {
					return n, errUnexpectedEOS
				}
			} else {
				return n, err
			}
		}
		if r.limited && r.buf.top == r.limit {
			err = r.close()
			if err != nil {
				return n, err
			}
		}
	}
}

func (r *Reader) setSize(size int64) error {
	limit := r.head + size
	if limit < r.buf.top {
		return errors.New("limit out of range")
	}
	r.limited = true
	r.limit = limit
	r.move(0)
	return nil
}

func (r *Reader) Restart(lzma io.Reader) {
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
