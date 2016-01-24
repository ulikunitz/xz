package lzma

import (
	"fmt"
	"io"
)

// opLenMargin provides the upper limit of the number of bytes required
// to encode a single operation.
const opLenMargin = 10

// compressFlags control the compression process.
type compressFlags uint32

// Values for compressFlags.
const (
	// all data should be compresed, even if compression is not
	// optimal.
	all compressFlags = 1 << iota
)

type findOps func(d *EncoderDict, f compressFlags) (end bool)

// EncoderFlags provide the flags for an encoder.
type EncoderFlags uint32

// Flags for the encoder.
const (
	// EOSMarker requests an EOS marker to be written.
	EOSMarker EncoderFlags = 1 << iota
)

// Encoder compresses data buffered in the encoder dictionary and writes
// it into a byte writer.
type Encoder struct {
	Dict  *EncoderDict
	State *State
	re    *rangeEncoder
	start int64
	// generate eos marker
	marker  bool
	limit   bool
	findOps findOps
	margin  int
}

// NewEncoder creates a new encoder. If the byte writer must be
// limited use LimitedByteWriter provided by this package. The flags
// argument supports the EOSMarker flag, controlling whether a
// terminating end-of-stream marker must be written.
func NewEncoder(bw io.ByteWriter, state *State, dict *EncoderDict,
	flags EncoderFlags) (e *Encoder, err error) {

	re, err := newRangeEncoder(bw)
	if err != nil {
		return nil, err
	}
	e = &Encoder{
		findOps: greedy,
		Dict:    dict,
		State:   state,
		re:      re,
		marker:  flags&EOSMarker != 0,
		start:   dict.pos(),
		margin:  opLenMargin,
	}
	e.Dict.reps = e.State.rep
	if e.marker {
		e.margin += 5
	}
	return e, nil
}

// Write writes the bytes from p into the dictionary. If not enough
// space is available the data in the dictionary buffer will be
// compressed to make additional space available. If the limit of the
// underlying writer has been reached ErrLimit will be returned.
func (e *Encoder) Write(p []byte) (n int, err error) {
	for {
		k, err := e.Dict.write(p[n:])
		n += k
		if err == ErrNoSpace {
			if err = e.compress(0); err != nil {
				return n, err
			}
			continue
		}
		return n, err
	}
}

// Reopen reopens the encoder with a new byte writer.
func (e *Encoder) Reopen(bw io.ByteWriter) error {
	var err error
	if e.re, err = newRangeEncoder(bw); err != nil {
		return err
	}
	e.start = e.Dict.pos()
	e.limit = false
	return nil
}

// writeLiteral writes a literal into the LZMA stream
func (e *Encoder) writeLiteral(l lit) error {
	var err error
	state, state2, _ := e.State.states(e.Dict.pos())
	if err = e.State.isMatch[state2].Encode(e.re, 0); err != nil {
		return err
	}
	litState := e.State.litState(e.Dict.byteAt(1), e.Dict.pos())
	match := e.Dict.byteAt(int(e.State.rep[0]) + 1)
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
		panic(fmt.Errorf(
			"match length %d out of range; dist %d rep[0] %d",
			m.n, dist, e.State.rep[0]))
	}
	state, state2, posState := e.State.states(e.Dict.pos())
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

// writeOps writes the buffered operations into the range encoder. If
// there is not enough space in the range encoder ErrLimit will be
// returned.
func (e *Encoder) writeOps() error {
	for e.Dict.ops.buffered() > 0 {
		if e.re.Available() < int64(e.margin) {
			return ErrLimit
		}
		op, err := e.Dict.peekOp()
		if err != nil {
			return err
		}
		switch x := op.(type) {
		case match:
			err = e.writeMatch(x)
		case lit:
			err = e.writeLiteral(x)
		}
		if err != nil {
			return err
		}
		if err = e.Dict.discardOp(); err != nil {
			return err
		}
	}
	return nil
}

// compress compressed data from the dictionary buffer. If the flag all
// is set, all data in the dictionay buffer will be compressed. The
// function returns ErrLimit if the underlying writer has reached its
// limit.
func (e *Encoder) compress(flags compressFlags) error {
	var end bool
	for {
		if err := e.writeOps(); err != nil {
			return err
		}
		if end {
			return nil
		}
		end = e.findOps(e.Dict, flags)
	}
}

// eosMatch is a pseudo operation that indicates the end of the stream.
var eosMatch = match{distance: maxDistance, n: minMatchLen}

// Close terminates the LZMA stream. If requested the end-of-stream
// marker will be written. If the byte writer limit has been or will be
// reached during compression of the remaining data in the buffer the
// LZMA stream will be closed and data will remain in the buffer.
func (e *Encoder) Close() error {
	err := e.compress(all)
	if err != nil && err != ErrLimit {
		return err
	}
	if e.marker {
		if err := e.writeMatch(eosMatch); err != nil {
			return err
		}
	}
	err = e.re.Close()
	return err
}

// Compressed returns the number bytes of the input data that been
// compressed.
func (e *Encoder) Compressed() int64 {
	return e.Dict.pos() - e.start
}
