package lzb

import (
	"errors"
	"io"
)

type opReader struct {
	state     *State
	buf       *buffer
	rd        *rangeDecoder
	pendingOp operation
	eos       bool
	closed    bool
}

func newOpReader(r io.Reader, state *State) (or *opReader, err error) {
	if _, ok := state.dict.(*syncDict); !ok {
		return nil, errors.New(
			"state must support a reader (no syncDict)")
	}
	or = &opReader{state: state, buf: state.dict.buffer()}
	if or.rd, err = newRangeDecoder(r); err != nil {
		return nil, err
	}
	return or, nil
}

// Errors produced by readOp and fillBuffer
var (
	eos              = errors.New("end of stream")
	errClosed        = errors.New("stream is closed")
	errDataAfterEOS  = errors.New("data after end of stream")
	errUnexpectedEOF = errors.New("unexpected end of compressed stream")
)

// decodeLiteral reads a literal.
func (or *opReader) decodeLiteral() (op operation, err error) {
	litState := or.state.litState()
	match := or.state.dict.byteAt(int64(or.state.rep[0]) + 1)
	s, err := or.state.litCodec.Decode(or.rd, or.state.state, match,
		litState)
	if err != nil {
		return nil, err
	}
	return lit{s}, nil
}

// readOp decodes the next operation from the compressed stream. It returns the
// operation. If an explicit end of stream marker is identified the eos error is
// returned.
func (or *opReader) readOp() (op operation, err error) {
	// Value of the end of stream (EOS) marker
	const eosDist = 1<<32 - 1

	state, state2, posState := or.state.states()

	b, err := or.state.isMatch[state2].Decode(or.rd)
	if err != nil {
		return nil, err
	}
	if b == 0 {
		// literal
		op, err := or.decodeLiteral()
		if err != nil {
			return nil, err
		}
		or.state.updateStateLiteral()
		return op, nil
	}
	b, err = or.state.isRep[state].Decode(or.rd)
	if err != nil {
		return nil, err
	}
	if b == 0 {
		// simple match
		or.state.rep[3], or.state.rep[2], or.state.rep[1] = or.state.rep[2], or.state.rep[1], or.state.rep[0]

		or.state.updateStateMatch()
		// The length decoder returns the length offset.
		n, err := or.state.lenCodec.Decode(or.rd, posState)
		if err != nil {
			return nil, err
		}
		// The dist decoder returns the distance offset. The actual
		// distance is 1 higher.
		or.state.rep[0], err = or.state.distCodec.Decode(or.rd, n)
		if err != nil {
			return nil, err
		}
		if or.state.rep[0] == eosDist {
			or.eos = true
			return nil, eos
		}
		op = match{n: int(n) + MinLength,
			distance: int64(or.state.rep[0]) + minDistance}
		return op, nil
	}
	b, err = or.state.isRepG0[state].Decode(or.rd)
	if err != nil {
		return nil, err
	}
	dist := or.state.rep[0]
	if b == 0 {
		// rep match 0
		b, err = or.state.isRepG0Long[state2].Decode(or.rd)
		if err != nil {
			return nil, err
		}
		if b == 0 {
			or.state.updateStateShortRep()
			op = match{n: 1, distance: int64(dist) + minDistance}
			return op, nil
		}
	} else {
		b, err = or.state.isRepG1[state].Decode(or.rd)
		if err != nil {
			return nil, err
		}
		if b == 0 {
			dist = or.state.rep[1]
		} else {
			b, err = or.state.isRepG2[state].Decode(or.rd)
			if err != nil {
				return nil, err
			}
			if b == 0 {
				dist = or.state.rep[2]
			} else {
				dist = or.state.rep[3]
				or.state.rep[3] = or.state.rep[2]
			}
			or.state.rep[2] = or.state.rep[1]
		}
		or.state.rep[1] = or.state.rep[0]
		or.state.rep[0] = dist
	}
	n, err := or.state.repLenCodec.Decode(or.rd, posState)
	if err != nil {
		return nil, err
	}
	or.state.updateStateRep()
	op = match{n: int(n) + MinLength, distance: int64(dist) + minDistance}
	return op, nil
}

func (or *opReader) close() error {
	if or.closed {
		return errClosed
	}
	if or.pendingOp != nil {
		return errDataAfterEOS
	}
	if !or.rd.possiblyAtEnd() {
		_, err := or.readOp()
		if err != eos {
			if err != nil {
				return err
			}
			return errDataAfterEOS
		}
		if !or.rd.possiblyAtEnd() {
			return errDataAfterEOS
		}
	}
	or.closed = true
	return nil
}

// fillBuffer fills the buffer with data read from the LZMA stream.
func (or *opReader) fillBuffer() error {
	if or.closed {
		return errClosed
	}
	d := or.state.dict.(*syncDict)
	if or.pendingOp != nil {
		op := or.pendingOp
		if or.buf.top+int64(op.Len()) > or.buf.writeLimit {
			return nil
		}
		if err := op.Apply(d); err != nil {
			return err
		}
		or.pendingOp = nil
	}
	for or.buf.top < or.buf.writeLimit {
		op, err := or.readOp()
		if err != nil {
			switch err {
			case eos:
				or.closed = true
				if !or.rd.possiblyAtEnd() {
					return errDataAfterEOS
				}
				return eos
			case io.EOF:
				or.closed = true
				return errUnexpectedEOF
			default:
				return err
			}
		}
		if or.buf.top+int64(op.Len()) > or.buf.writeLimit {
			or.pendingOp = op
			return nil
		}
		if err = op.Apply(d); err != nil {
			return err
		}
	}
	return nil
}
