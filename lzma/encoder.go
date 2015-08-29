package lzma

import (
	"errors"
	"fmt"
	"io"
)

const opLenMargin = 10

// opFinder enables the support of multiple different OpFinder
// algorithms.
type opFinder interface {
	findOps(e *encoderDict, r reps, all bool) []operation
	name() string
}

// Encoder supports the compression of uncompressed data into a raw LZMA
// stream.
type Encoder struct {
	buf              encoderBuffer
	dict             encoderDict
	state            state
	re               *rangeEncoder
	start            int64
	uncompressedSize int64
	flags            CFlags
	opFinder         opFinder
	margin           int
	// uncompressed
	w io.Writer
}

func InitEncoder(e *Encoder, w io.Writer, p *CodecParams) error {
	*e = Encoder{opFinder: greedyFinder{}}

	var err error
	if err = initBuffer(&e.buf.buffer, p.BufCap); err != nil {
		return err
	}
	if e.buf.matcher, err = newHashTable(p.BufCap, 4); err != nil {
		return err
	}
	if err = initEncoderDict(&e.dict, p.DictCap, &e.buf); err != nil {
		return err
	}

	p.Flags |= CResetDict
	err = e.Reset(w, p)
	return err
}

// NewEncoder initializes a new encoder.
//
// The parameters CompressedSize and UncompressedSize have the functions
// of limits for the amount of data to be compressed or uncompressed.
func NewEncoder(w io.Writer, p *CodecParams) (e *Encoder, err error) {
	e = new(Encoder)
	if err = InitEncoder(e, w, p); err != nil {
		return nil, err
	}
	return e, nil
}

// Reset reinitializes the encoder with a new writer. Data that has not
// been compressed so far will remain to be stored in the buffer. The
// buffer capacity and dictionary capacity will not be changed.
func (e *Encoder) Reset(w io.Writer, p *CodecParams) error {
	e.flags = p.Flags
	if p.Flags&CNoUncompressedSize != 0 {
		e.uncompressedSize = maxInt64
	} else {
		e.uncompressedSize = p.UncompressedSize
	}

	if p.Flags&CResetDict != 0 {
		e.dict.Reset()
	}
	e.start = e.dict.Pos()

	if p.Flags&(CResetProperties|CResetDict) != 0 {
		props, err := NewProperties(p.LC, p.LP, p.PB)
		if err != nil {
			return err
		}
		initState(&e.state, props)
	} else if p.Flags&CResetState != 0 {
		e.state.Reset()
	}

	if p.Flags&CUncompressed != 0 {
		e.w = w
		return nil
	}

	var err error
	if p.Flags&CNoCompressedSize != 0 {
		e.re, err = newRangeEncoder(w)
	} else {
		e.re, err = newRangeEncoderLimit(w, p.CompressedSize)
	}
	if err != nil {
		return err
	}

	e.margin = opLenMargin
	if e.flags&CEOSMarker != 0 {
		e.margin += 5
	}
	return nil
}

// ErrCompressedLimit and ErrUncompressedLimit indicate that the provided
// sizes have been reached. The encoder must be closed and reset to
// compress the remaining buffered data.
var (
	ErrCompressedLimit   = errors.New("compressed size limit reached")
	ErrUncompressedLimit = errors.New("uncompressed size limit reached")
)

// Write writes the provided bytes into the buffer. If the buffer is
// full Write will compress the data. The full data may not be written
// if either the compressed size or the uncompressed size limit have
// been reached.
func (e *Encoder) Write(p []byte) (n int, err error) {
	if e.flags&CUncompressed != 0 {
		m := e.uncompressedSize - e.Uncompressed()
		if m < int64(len(p)) {
			p = p[:m]
			err = ErrUncompressedLimit
		}
		var werr error
		n, werr = e.w.Write(p)
		if werr != nil {
			err = werr
		}
		p = p[:n]
		var j int
		for j < n {
			k, werr := e.buf.Write(p[j:])
			j += k
			if werr != nil && werr != errNoSpace {
				panic(fmt.Errorf("buf.Write: unexpected %s",
					werr))
			}
			e.dict.Advance(k)
		}
		return n, err
	}
	for {
		k, err := e.buf.Write(p[n:])
		n += k
		if err != errNoSpace {
			return n, err
		}
		if err = e.compress(false); err != nil {
			return n, err
		}
	}
}

// writeLiteral writes a literal into the LZMA stream
func (e *Encoder) writeLiteral(l lit) error {
	var err error
	state, state2, _ := e.state.states(e.dict.head)
	if err = e.state.isMatch[state2].Encode(e.re, 0); err != nil {
		return err
	}
	litState := e.state.litState(e.dict.ByteAt(1), e.dict.head)
	match := e.dict.ByteAt(int(e.state.rep[0]) + 1)
	err = e.state.litCodec.Encode(e.re, l.b, state, match, litState)
	if err != nil {
		return err
	}
	e.state.updateStateLiteral()
	return nil
}

// iverson implements the Iverson operator as proposed by Donald Knuth in his
// book Concrete Mathematics.
func iverson(ok bool) uint32 {
	if ok {
		return 1
	}
	return 0
}

// writeMatch writes a repetition operation into the operation stream
func (e *Encoder) writeMatch(m match) error {
	var err error
	if !(minDistance <= m.distance && m.distance <= maxDistance) {
		panic("match distance out of range")
	}
	dist := uint32(m.distance - minDistance)
	if !(MinMatchLen <= m.n && m.n <= MaxMatchLen) &&
		!(dist == e.state.rep[0] && m.n == 1) {
		panic("match length out of range")
	}
	state, state2, posState := e.state.states(e.dict.head)
	if err = e.state.isMatch[state2].Encode(e.re, 1); err != nil {
		return err
	}
	g := 0
	for ; g < 4; g++ {
		if e.state.rep[g] == dist {
			break
		}
	}
	b := iverson(g < 4)
	if err = e.state.isRep[state].Encode(e.re, b); err != nil {
		return err
	}
	n := uint32(m.n - MinMatchLen)
	if b == 0 {
		// simple match
		e.state.rep[3], e.state.rep[2], e.state.rep[1], e.state.rep[0] =
			e.state.rep[2], e.state.rep[1], e.state.rep[0], dist
		e.state.updateStateMatch()
		if err = e.state.lenCodec.Encode(e.re, n, posState); err != nil {
			return err
		}
		return e.state.distCodec.Encode(e.re, dist, n)
	}
	b = iverson(g != 0)
	if err = e.state.isRepG0[state].Encode(e.re, b); err != nil {
		return err
	}
	if b == 0 {
		// g == 0
		b = iverson(m.n != 1)
		if err = e.state.isRepG0Long[state2].Encode(e.re, b); err != nil {
			return err
		}
		if b == 0 {
			e.state.updateStateShortRep()
			return nil
		}
	} else {
		// g in {1,2,3}
		b = iverson(g != 1)
		if err = e.state.isRepG1[state].Encode(e.re, b); err != nil {
			return err
		}
		if b == 1 {
			// g in {2,3}
			b = iverson(g != 2)
			err = e.state.isRepG2[state].Encode(e.re, b)
			if err != nil {
				return err
			}
			if b == 1 {
				e.state.rep[3] = e.state.rep[2]
			}
			e.state.rep[2] = e.state.rep[1]
		}
		e.state.rep[1] = e.state.rep[0]
		e.state.rep[0] = dist
	}
	e.state.updateStateRep()
	return e.state.repLenCodec.Encode(e.re, n, posState)
}

func (e *Encoder) sanityCheck(s string) {
	if e.buf.Buffered() != e.dict.Buffered()+e.dict.Len() {
		fmt.Printf("%s buffered %d; want %d (%d+%d) cap %d\n",
			s, e.buf.Buffered(), e.dict.Buffered()+e.dict.Len(),
			e.dict.Buffered(), e.dict.Len(), e.dict.capacity)
		panic("inconsistent buffer sizes")
	}
}

func (e *Encoder) discardOp(op operation) error {
	e.sanityCheck("#1")
	n := op.Len()
	m := n + e.dict.Len() - e.dict.capacity
	if m > 0 {
		if _, err := e.buf.Discard(m); err != nil {
			return err
		}
	}
	e.dict.Advance(n)
	e.sanityCheck("#2")
	return nil
}

// writeOp writes an operation value into the stream. It checks whether there
// is still enough space available using an upper limit for the size required.
func (e *Encoder) writeOp(op operation) error {
	if e.re.Available() < int64(e.margin) {
		return ErrCompressedLimit
	}
	if e.uncompressedSize-e.Uncompressed() < int64(op.Len()) {
		return ErrUncompressedLimit
	}
	var err error
	switch x := op.(type) {
	case match:
		err = e.writeMatch(x)
	case lit:
		err = e.writeLiteral(x)
	}
	if err != nil {
		return err
	}
	err = e.discardOp(op)
	return err
}

func (e *Encoder) compress(all bool) error {
	ops := e.opFinder.findOps(&e.dict, reps(e.state.rep), all)
	for _, op := range ops {
		if err := e.writeOp(op); err != nil {
			return err
		}
	}
	return nil
}

// eosMatch is a pseudo operation that indicates the end of the stream.
var eosMatch = match{distance: maxDistance, n: MinMatchLen}

// Close tries to write the outstanding data in the buffer to the
// underlying writer until compressed or uncompressed size limits areif
// reached. In any case the LZMA stream will be correctly closed and no
// error will be returned. If there is remaining data in the buffer the
// encoder needs to be reset.
func (e *Encoder) Close() error {
	if e.flags&CUncompressed != 0 {
		return nil
	}
	err := e.compress(true)
	if err != nil && err != ErrUncompressedLimit && err != ErrCompressedLimit {

		return err
	}
	if e.flags&CEOSMarker != 0 {
		if err := e.writeMatch(eosMatch); err != nil {
			return err
		}
	}
	return e.re.Close()
}

// Buffered reports the amount of data that has not been written to the
// underlying writer.
func (e *Encoder) Buffered() int {
	return e.dict.Buffered()
}

// Compressed returns the number of bytes that have been written to
// the underlying writer.
func (e *Encoder) Compressed() int64 {
	return e.re.Compressed()
}

// Uncompressed provides the amount of data that has already been
// compressed to the underlying data stream.
func (e *Encoder) Uncompressed() int64 {
	return e.dict.Pos() - e.start
}
