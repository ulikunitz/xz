package lzma

import "io"

const opLenMargin = 10

// opFinder enables the support of multiple different OpFinder
// algorithms.
type opFinder interface {
	findOps(e *EncoderDict, r reps, all bool) []operation
	name() string
}

// Encoder supports the compression of uncompressed data into a raw LZMA
// stream.
type Encoder struct {
	Dict       *EncoderDict
	State      *State
	writerDict writerDict
	re         *rangeEncoder
	start      int64
	eosMarker  bool
	opFinder   opFinder
	margin     int
}

// Init sets the encoder up for use. If the byte writer must be limited use
// LimitedByteWriter provided by this package.
func (e *Encoder) Init(bw io.ByteWriter, state *State, dict *EncoderDict,
	p CodecParams) error {
	*e = Encoder{opFinder: greedyFinder{}}
	e.Dict = dict
	e.State = state
	var err error
	if e.re, err = newRangeEncoder(bw); err != nil {
		return err
	}

	e.eosMarker = p.EOSMarker
	e.start = e.Dict.Pos()

	e.margin = opLenMargin
	if e.eosMarker {
		e.margin += 5
	}
	return nil
}

// Write writes the provided bytes into the buffer. If the buffer is
// full Write will compress the data. The full data may not be written
// if either the compressed size or the uncompressed size limit have
// been reached.
func (e *Encoder) Write(p []byte) (n int, err error) {
	for {
		k, err := e.Dict.Write(p[n:])
		n += k
		if err != ErrNoSpace {
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
	state, state2, _ := e.State.states(e.writerDict.Pos())
	if err = e.State.isMatch[state2].Encode(e.re, 0); err != nil {
		return err
	}
	litState := e.State.litState(e.writerDict.ByteAt(1), e.writerDict.Pos())
	match := e.writerDict.ByteAt(int(e.State.rep[0]) + 1)
	err = e.State.litCodec.Encode(e.re, l.b, state, match, litState)
	if err != nil {
		return err
	}
	e.State.updateStateLiteral()
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
	if !(minMatchLen <= m.n && m.n <= maxMatchLen) &&
		!(dist == e.State.rep[0] && m.n == 1) {
		panic("match length out of range")
	}
	state, state2, posState := e.State.states(e.writerDict.Pos())
	if err = e.State.isMatch[state2].Encode(e.re, 1); err != nil {
		return err
	}
	g := 0
	for ; g < 4; g++ {
		if e.State.rep[g] == dist {
			break
		}
	}
	b := iverson(g < 4)
	if err = e.State.isRep[state].Encode(e.re, b); err != nil {
		return err
	}
	n := uint32(m.n - minMatchLen)
	if b == 0 {
		// simple match
		e.State.rep[3], e.State.rep[2], e.State.rep[1], e.State.rep[0] =
			e.State.rep[2], e.State.rep[1], e.State.rep[0], dist
		e.State.updateStateMatch()
		if err = e.State.lenCodec.Encode(e.re, n, posState); err != nil {
			return err
		}
		return e.State.distCodec.Encode(e.re, dist, n)
	}
	b = iverson(g != 0)
	if err = e.State.isRepG0[state].Encode(e.re, b); err != nil {
		return err
	}
	if b == 0 {
		// g == 0
		b = iverson(m.n != 1)
		if err = e.State.isRepG0Long[state2].Encode(e.re, b); err != nil {
			return err
		}
		if b == 0 {
			e.State.updateStateShortRep()
			return nil
		}
	} else {
		// g in {1,2,3}
		b = iverson(g != 1)
		if err = e.State.isRepG1[state].Encode(e.re, b); err != nil {
			return err
		}
		if b == 1 {
			// g in {2,3}
			b = iverson(g != 2)
			err = e.State.isRepG2[state].Encode(e.re, b)
			if err != nil {
				return err
			}
			if b == 1 {
				e.State.rep[3] = e.State.rep[2]
			}
			e.State.rep[2] = e.State.rep[1]
		}
		e.State.rep[1] = e.State.rep[0]
		e.State.rep[0] = dist
	}
	e.State.updateStateRep()
	return e.State.repLenCodec.Encode(e.re, n, posState)
}

// writeOp writes an operation value into the stream. It checks whether there
// is still enough space available using an upper limit for the size required.
func (e *Encoder) writeOp(op operation) error {
	if e.re.Available() < int64(e.margin) {
		return ErrLimit
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
	_, err = e.writerDict.Advance(op.Len())
	return err
}

// Wash compresses all data in the buffer. It isn't able to write out
// the complete range encoder status, this isn't supported by the LZMA
// format.
func (e *Encoder) Wash() error {
	return e.compress(true)
}

// compress compresses code available in the buffer and writes the
// operation into the encoder.
func (e *Encoder) compress(all bool) error {
	e.writerDict = e.Dict.writerDict
	ops := e.opFinder.findOps(e.Dict, reps(e.State.rep), all)
	for _, op := range ops {
		if err := e.writeOp(op); err != nil {
			return err
		}
	}
	return nil
}

// eosMatch is a pseudo operation that indicates the end of the stream.
var eosMatch = match{distance: maxDistance, n: minMatchLen}

// Close closes the stream without writing the outstanding data in the
// buffer.
func (e *Encoder) Close() error {
	if e.eosMarker {
		if err := e.writeMatch(eosMatch); err != nil {
			return err
		}
	}
	return e.re.Close()
}

// Uncompressed provides the amount of data that has already been
// compressed to the underlying data stream.
func (e *Encoder) Uncompressed() int64 {
	return e.Dict.Pos() - e.start
}
