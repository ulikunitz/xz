package lzbase

import "io"

// Writer supports the creation of an LZMA stream.
type Writer struct {
	State *WriterState
	lw    *LimitedWriter
	re    *rangeEncoder
	dict  *WriterDict
	eos   bool
}

// NewWriter creates a writer using the state. The argument eos defines whether
// an explicit end-of-stream marker will be written. The writer will be limited
// by MaxLimit (2^63 - 1), which is practically unlimited.
func NewWriter(w io.Writer, state *WriterState, eos bool) *Writer {
	lw := &LimitedWriter{W: w, N: maxLimit}
	return &Writer{
		State: state,
		lw:    lw,
		re:    newRangeEncoder(lw),
		dict:  state.WriterDict(),
		eos:   eos}
}

// Limit returns the number of byte that can still be written at maximum.
func (bw *Writer) Limit() int64 {
	return bw.lw.N
}

// SetLimit sets the number of bytes that can be written at maximum.
func (bw *Writer) SetLimit(n int64) {
	bw.lw.N = n
}

// Write moves data into the internal buffer and triggers its compression. Note
// that beside the data held back to enable a large match all data will be be
// compressed.
func (bw *Writer) Write(p []byte) (n int, err error) {
	end := bw.dict.end + int64(len(p))
	if end < 0 {
		panic("end counter overflow")
	}
	for n < len(p) {
		k, err := bw.dict.Write(p[n:])
		n += k
		if err != nil && err != errAgain {
			return n, err
		}
		if err = bw.process(0); err != nil {
			return n, err
		}
	}
	return n, nil
}

// Close terminates the LZMA stream. It doesn't close the underlying writer
// though and leaves it alone. In some scenarios explicit closing of the
// underlying writer is required.
func (bw *Writer) Close() error {
	var err error
	if err = bw.process(allData); err != nil {
		return err
	}
	if bw.eos {
		if err = bw.writeMatch(eosMatch); err != nil {
			return err
		}
	}
	if err = bw.re.Close(); err != nil {
		return err
	}
	return bw.dict.closeBuffer()
}

// The allData flag tells the process method that all data must be processed.
const allData = 1

// process encodes the data written into the dictionary buffer. The allData
// flag requires all data remaining in the buffer to be encoded.
func (bw *Writer) process(flags int) error {
	var lowMark int
	if flags&allData == 0 {
		lowMark = MaxLength - 1
	}
	for bw.dict.readable() > lowMark {
		op, err := bw.dict.FindOp(bw.State)
		if err != nil {
			debug.Printf("findOp error %s\n", err)
			return err
		}
		if err = bw.writeOp(op); err != nil {
			return err
		}
		debug.Printf("op %s", op)
		if err = bw.dict.DiscardOp(op); err != nil {
			return err
		}
	}
	return nil
}

// writeLiteral writes a literal into the operation stream
func (bw *Writer) writeLiteral(l lit) error {
	var err error
	state, state2, _ := bw.State.states()
	if err = bw.State.isMatch[state2].Encode(bw.re, 0); err != nil {
		return err
	}
	litState := bw.State.litState()
	match := bw.dict.byteAt(int64(bw.State.rep[0]) + 1)
	err = bw.State.litCodec.Encode(bw.re, l.b, state, match, litState)
	if err != nil {
		return err
	}
	bw.State.updateStateLiteral()
	return nil
}

func iverson(ok bool) uint32 {
	if ok {
		return 1
	}
	return 0
}

// writeMatch writes a repetition operation into the operation stream
func (bw *Writer) writeMatch(m match) error {
	var err error
	if !(minDistance <= m.distance && m.distance <= maxDistance) {
		return newError("distance out of range")
	}
	dist := uint32(m.distance - minDistance)
	if !(MinLength <= m.n && m.n <= MaxLength) &&
		!(dist == bw.State.rep[0] && m.n == 1) {
		return newError("length out of range")
	}
	state, state2, posState := bw.State.states()
	if err = bw.State.isMatch[state2].Encode(bw.re, 1); err != nil {
		return err
	}
	var g int
	for g = 0; g < 4; g++ {
		if bw.State.rep[g] == dist {
			break
		}
	}
	b := iverson(g < 4)
	if err = bw.State.isRep[state].Encode(bw.re, b); err != nil {
		return err
	}
	n := uint32(m.n - MinLength)
	if b == 0 {
		// simple match
		bw.State.rep[3], bw.State.rep[2], bw.State.rep[1], bw.State.rep[0] = bw.State.rep[2],
			bw.State.rep[1], bw.State.rep[0], dist
		bw.State.updateStateMatch()
		if err = bw.State.lenCodec.Encode(bw.re, n, posState); err != nil {
			return err
		}
		return bw.State.distCodec.Encode(bw.re, dist, n)
	}
	b = iverson(g != 0)
	if err = bw.State.isRepG0[state].Encode(bw.re, b); err != nil {
		return err
	}
	if b == 0 {
		// g == 0
		b = iverson(m.n != 1)
		if err = bw.State.isRepG0Long[state2].Encode(bw.re, b); err != nil {
			return err
		}
		if b == 0 {
			bw.State.updateStateShortRep()
			return nil
		}
	} else {
		// g in {1,2,3}
		b = iverson(g != 1)
		if err = bw.State.isRepG1[state].Encode(bw.re, b); err != nil {
			return err
		}
		if b == 1 {
			// g in {2,3}
			b = iverson(g != 2)
			err = bw.State.isRepG2[state].Encode(bw.re, b)
			if err != nil {
				return err
			}
			if b == 1 {
				bw.State.rep[3] = bw.State.rep[2]
			}
			bw.State.rep[2] = bw.State.rep[1]
		}
		bw.State.rep[1] = bw.State.rep[0]
		bw.State.rep[0] = dist
	}
	bw.State.updateStateRep()
	return bw.State.repLenCodec.Encode(bw.re, n, posState)
}

// maxOpSize gives an upper limit for the number of bytes a single operation
// might require.
const maxOpSize = 7

// writeOp writes an operation value into the stream. It checks whether there
// is still enough space available using an upper limit for the size required.
func (bw *Writer) writeOp(op Operation) error {
	if bw.lw.N < bw.re.closeLen()+maxOpSize {
		return Limit
	}
	switch x := op.(type) {
	case match:
		return bw.writeMatch(x)
	case lit:
		return bw.writeLiteral(x)
	}
	panic("unknown operation type")
}
