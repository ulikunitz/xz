package lzbase

import "io"

// OpEncoder finds and encodes operations.
type OpEncoder struct {
	State *WriterState
	dict  *WriterDict
	re    *rangeEncoder
	eos   bool
}

// NewOpEncoder creates a new op encoder instance. The eos argument controls
// whether an EOS marker will be written to mark the end of the LZMA stream.
func NewOpEncoder(w io.Writer, state *WriterState, eos bool) *OpEncoder {
	return &OpEncoder{
		State: state,
		dict:  state.WriterDict(),
		re:    newRangeEncoder(w),
		eos:   eos}
}

// writeLiteral writes a literal into the operation stream
func (e *OpEncoder) writeLiteral(l lit) error {
	var err error
	state, state2, _ := e.State.states()
	if err = e.State.isMatch[state2].Encode(e.re, 0); err != nil {
		return err
	}
	litState := e.State.litState()
	match := e.dict.byteAt(int64(e.State.rep[0]) + 1)
	err = e.State.litCodec.Encode(e.re, l.b, state, match, litState)
	if err != nil {
		return err
	}
	e.State.updateStateLiteral()
	return nil
}

// iversion implements the Iverson operator as proposed by Donald Knuth in his
// book Concrete Mathematics.
func iverson(ok bool) uint32 {
	if ok {
		return 1
	}
	return 0
}

// writeMatch writes a repetition operation into the operation stream
func (e *OpEncoder) writeMatch(m match) error {
	var err error
	if !(minDistance <= m.distance && m.distance <= maxDistance) {
		return newError("distance out of range")
	}
	dist := uint32(m.distance - minDistance)
	if !(MinLength <= m.n && m.n <= MaxLength) &&
		!(dist == e.State.rep[0] && m.n == 1) {
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
	n := uint32(m.n - MinLength)
	if b == 0 {
		// simple match
		e.State.rep[3], e.State.rep[2], e.State.rep[1], e.State.rep[0] = e.State.rep[2],
			e.State.rep[1], e.State.rep[0], dist
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

// Write writes an operation value into the stream. It checks whether there
// is still enough space available using an upper limit for the size required.
func (e *OpEncoder) Write(op Operation) error {
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
	return e.discard(op)
}

// Close closes the encoder and writes the EOS marker if required.
func (e *OpEncoder) Close() error {
	if e.eos {
		if err := e.writeMatch(eosMatch); err != nil {
			return err
		}
	}
	if err := e.re.Close(); err != nil {
		return err
	}
	return e.dict.closeBuffer()
}

// potentialOffsets creates a list of potential offsets for matches.
func (e *OpEncoder) potentialOffsets(p []byte) []int64 {
	head := e.dict.offset()
	start := e.dict.start
	offs := make([]int64, 0, 32)
	// add potential offsets with highest priority at the top
	for i := 1; i < 11; i++ {
		// distance 1 to 10
		off := head - int64(i)
		if start <= off {
			offs = append(offs, off)
		}
	}
	if len(p) == 4 {
		// distances from the hash table
		offs = append(offs, e.dict.t4.Offsets(p)...)
	}
	for i := 3; i >= 0; i-- {
		// distances from the repetition for length less than 4
		dist := int64(e.State.rep[i]) + minDistance
		off := head - dist
		if start <= off {
			offs = append(offs, off)
		}
	}
	return offs
}

// errNoMatch indicates that no match could be found
var errNoMatch = newError("no match found")

// bestMatch finds the best match for the given offsets.
//
// TODO: compare all possible commands for compressed bits per encoded bits.
func (e *OpEncoder) bestMatch(offsets []int64) (m match, err error) {
	// creates a match for 1
	head := e.dict.offset()
	off := int64(-1)
	length := 0
	for i := len(offsets) - 1; i >= 0; i-- {
		n := e.dict.equalBytes(head, offsets[i], MaxLength)
		if n > length {
			off, length = offsets[i], n
		}
	}
	if off < 0 {
		err = errNoMatch
		return
	}
	if length == 1 {
		dist := int64(e.State.rep[0]) + minDistance
		offRep0 := head - dist
		if off != offRep0 {
			err = errNoMatch
			return
		}
	}
	return match{distance: head - off, n: length}, nil
}

// errEmptyBuf indicates an empty buffer
var errEmptyBuf = newError("empty buffer")

// Find finds an operation for the head of the dictionary.
//
// TODO: Make this function internal and use another interface that returns a
// list of operations.
func (e *OpEncoder) Find() (op Operation, err error) {
	p := make([]byte, 4)
	n, err := e.dict.peekHead(p)
	if err != nil && err != errAgain && err != io.EOF {
		return nil, err
	}
	if n <= 0 {
		if n < 0 {
			panic("strange n")
		}
		return nil, errEmptyBuf
	}
	offs := e.potentialOffsets(p[:n])
	m, err := e.bestMatch(offs)
	if err == errNoMatch {
		return lit{b: p[0]}, nil
	}
	if err != nil {
		return nil, err
	}
	return m, nil
}

// discard advances the head of the dictionary and writes the respective
// bytes into the hash table of the dictionary.
func (e *OpEncoder) discard(op Operation) error {
	oplen := op.Len()
	n, err := e.dict.copyTo(e.dict.t4, oplen)
	if err != nil {
		return err
	}
	if n < oplen {
		return errAgain
	}
	return nil
}
