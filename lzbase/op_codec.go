package lzbase

import (
	"fmt"
	"io"
	"unicode"
)

// operation represents an operation on the dictionary during encoding or
// decoding.
type Operation interface {
	Len() int
}

// rep represents a repetition at the given distance and the given length
type match struct {
	// supports all possible distance values, including the eos marker
	distance int64
	length   int
}

// eos is a special kind of match.
var eos = match{distance: maxDistance, length: MinLength}

// EOS may mark the end of an LZMA stream.
var EOS = Operation(eos)

// Len return the length of the repetition.
func (m match) Len() int {
	return m.length
}

// String returns a string representation for the repetition.
func (m match) String() string {
	return fmt.Sprintf("match{%d,%d}", m.distance, m.length)
}

// lit represents a single byte literal.
type lit struct {
	b byte
}

// Len returns 1 for the single byte literal.
func (l lit) Len() int {
	return 1
}

// String returns a string representation for the literal.
func (l lit) String() string {
	var c byte
	if unicode.IsPrint(rune(l.b)) {
		c = l.b
	} else {
		c = '.'
	}
	return fmt.Sprintf("lit{%02x %c}", l.b, c)
}

// OpEncoder translates a sequences of operations to a byte stream.
type OpEncoder struct {
	W     io.Writer
	State *State
	re    *rangeEncoder
}

// NewOpEncoder creates a new OpEncoder value. Writer and state cannot be
// shared with other instances.
func NewOpEncoder(w io.Writer, state *State) (e *OpEncoder, err error) {
	switch {
	case w == nil:
		return nil, newError("NewOpEncoder argument w is nil")
	case state == nil:
		return nil, newError("NewOpEncoder argument state is nil")
	}
	e = &OpEncoder{
		W:     w,
		State: state,
		re:    newRangeEncoder(w),
	}
	return e, nil
}

// iverson translates a boolean into an integer value. Donald Knuth calls a
// mathematical operator doing the same Iverson operator in Concrete
// Mathematics.
func iverson(ok bool) uint32 {
	if ok {
		return 1
	}
	return 0
}

// writeMatch writes a match operation into the range encoder.
func (e *OpEncoder) writeMatch(m match) error {
	var err error
	if !(minDistance <= m.distance && m.distance <= maxDistance) {
		return newError("distance out of range")
	}
	dist := uint32(m.distance - minDistance)
	if !(MinLength <= m.length && m.length <= MaxLength) &&
		!(dist == e.State.rep[0] && m.length == 1) {
		return newError("length out of range")
	}
	state, state2, posState := e.State.states()
	if err = e.State.isMatch[state2].Encode(e.re, 1); err != nil {
		return err
	}
	var g int
	for g = 0; g < 4; g++ {
		if e.State.rep[g] == dist {
			break
		}
	}
	b := iverson(g < 4)
	if err = e.State.isRep[state].Encode(e.re, b); err != nil {
		return err
	}
	n := uint32(m.length - MinLength)
	if b == 0 {
		// simple match
		e.State.rep[3], e.State.rep[2], e.State.rep[1], e.State.rep[0] = e.State.rep[2], e.State.rep[1], e.State.rep[0], dist
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
		b = iverson(m.length != 1)
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

// writeLiteral writes a literal into the operation stream
func (e *OpEncoder) writeLiteral(l lit) error {
	var err error
	state, state2, _ := e.State.states()
	if err = e.State.isMatch[state2].Encode(e.re, 0); err != nil {
		return err
	}
	litState := e.State.litState()
	match := e.State.dict.Byte(int64(e.State.rep[0]) + 1)
	err = e.State.litCodec.Encode(e.re, l.b, state, match, litState)
	if err != nil {
		return err
	}
	e.State.updateStateLiteral()
	return nil
}

// WriteOps translates the given operations into an encoded byte stream. The
// number of operations written will be reported and any error condition. Note
// that an error might indicate that parts of the operation have already been
// written.
func (e *OpEncoder) WriteOps(ops []Operation) (n int, err error) {
	for _, op := range ops {
		switch x := op.(type) {
		case match:
			if err = e.writeMatch(x); err != nil {
				return n, err
			}
		case lit:
			if err = e.writeLiteral(x); err != nil {
				return n, err
			}
		default:
			return n, newError("unknown operation type")
		}
		n++
	}
	return n, nil
}

// Close closes the encoder.
func (e *OpEncoder) Close() error {
	return e.re.Close()
}
