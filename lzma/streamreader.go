package lzma

import (
	"io"

	"github.com/uli-go/xz/basics/i64"
)

type Reader struct {
	Params Parameters
	state  *State
	rd     *rangeDecoder
	buf    *buffer
	// head indicates the reading head
	head int64
	// start marks the offset at the start of the stream
	start int64
	// limit marks the expected size of the decompressed byte stream
	limit int64
	// closed marks readers where no more data will be written into
	// the buffer or dictionary
	closed bool
}

func (r *Reader) move(n int) {
	off, overflow := i64.Add(r.head, int64(n))
	if overflow {
		panic(errInt64Overflow)
	}
	if !(r.buf.bottom <= off && off <= r.buf.top) {
		panic("new offset out of range")
	}
	var limit int64
	limit, overflow = i64.Add(off, int64(r.buf.capacity()))
	if overflow {
		panic(errInt64Overflow)
	}
	if r.Params.SizeInHeader && limit > r.limit {
		limit = r.limit
	}
	if limit < r.buf.top {
		panic("limit out of range")
	}
	r.head = off
	r.buf.writeLimit = limit
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
	r = &Reader{
		Params: p,
		state:  state,
		rd:     rd,
		buf:    buf,
		head:   buf.bottom,
		start:  buf.bottom,
	}
	if p.SizeInHeader {
		r.limit = add(r.head, p.Size)
		if r.limit < r.buf.top {
			return nil, errReadLimit
		}
	}
	r.move(0)
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
			if !r.rd.possiblyAtEnd() {
				return nil, errDataAfterEOS
			}
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

// fillBuffer fills the buffer with data read from the LZMA stream.
func (r *Reader) fillBuffer() error {
	if r.closed {
		return nil
	}
	d := r.state.dict.(*syncDict)
	for {
		off := add(r.buf.top, MaxLength)
		if r.Params.SizeInHeader && off > r.limit {
			off = r.limit
		}
		if off > r.buf.writeLimit {
			return nil
		}
		op, err := r.readOp()
		switch err {
		case nil:
			break
		case eos:
			r.closed = true
			if r.Params.SizeInHeader && r.buf.top != r.limit {
				return errUnexpectedEOS
			}
			return nil
		case io.EOF:
			r.closed = true
			return io.ErrUnexpectedEOF
		default:
			return err
		}
		if err = op.Apply(d); err != nil {
			return err
		}
		if r.Params.SizeInHeader && r.buf.top >= r.limit {
			if r.buf.top > r.limit {
				panic("r.buf.top must not exceed r.limit here")
			}
			r.closed = true
			if !r.rd.possiblyAtEnd() {
				switch _, err = r.readOp(); err {
				case eos:
					if !r.rd.possiblyAtEnd() {
						return errDataAfterEOS
					}
					return nil
				case nil:
					return errDataAfterEOS
				default:
					return err
				}
			}
			return nil
		}
	}
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
			return n, err
		}
		if k >= len(p) {
			return n, nil
		}
		p = p[k:]
		if err = r.fillBuffer(); err != nil {
			return n, err
		}
	}
}

func (r *Reader) _readByte() (c byte, err error) {
	c, err = r.buf.readByteAt(r.head)
	switch err {
	case nil:
		r.move(1)
		return c, nil
	case errAgain:
		if r.closed {
			return 0, io.EOF
		}
		return 0, errAgain
	default:
		return 0, err
	}
}

func (r *Reader) ReadByte() (c byte, err error) {
	c, err = r._readByte()
	if err == nil || err != errAgain {
		return
	}
	if err = r.fillBuffer(); err != nil {
		return 0, err
	}
	c, err = r._readByte()
	if err == nil || err != errAgain {
		return
	}
	panic("couldn't read data")
}

func (r *Reader) WriteTo(w io.Writer) (n int64, err error) {
	if r.eof() {
		return 0, nil
	}
	for {
		var k int
		k, err = r.buf.writeRangeTo(r.head, r.buf.top, w)
		n += int64(k)
		if err != nil {
			return
		}
		r.move(k)
		if r.eof() {
			return n, nil
		}
		if err = r.fillBuffer(); err != nil {
			return n, err
		}
	}
}

func (r *Reader) Size() int64 {
	return r.head - r.start
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
