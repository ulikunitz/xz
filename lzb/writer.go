package lzb

import (
	"errors"
	"fmt"
	"io"
)

// Interface allowing the support of multiple operation finder
// algorithms.
type OpFinder interface {
	findOps(s *State, all bool) ([]operation, error)
	fmt.Stringer
}

// Writer produces an LZMA stream. EOS requests Close to write an
// end-of-stream marker.
type Writer struct {
	State    *State
	EOS      bool
	OpFinder OpFinder
	re       *rangeEncoder
	buf      *buffer
	closed   bool
}

// NewWriter creates a new writer instance.
func NewWriter(pw io.Writer, p Parameters) (w *Writer, err error) {
	if err = verifyParameters(&p); err != nil {
		return
	}
	buf, err := newBuffer(p.BufferSize + p.DictSize)
	if err != nil {
		return nil, err
	}
	d, err := newHashDict(buf, p.DictSize)
	if err != nil {
		return nil, err
	}
	d.sync()
	state := NewState(p.Properties(), d)
	return NewWriterState(pw, state)
}

// NewWriterState creates a new writer using an existing state.
func NewWriterState(pw io.Writer, state *State) (w *Writer, err error) {
	if _, ok := state.dict.(*hashDict); !ok {
		return nil, errors.New(
			"state must support a writer (no hashDict)")
	}
	w = &Writer{
		State:    state,
		buf:      state.dict.buffer(),
		re:       newRangeEncoder(pw),
		OpFinder: Greedy,
	}
	return w, nil
}

// writeLiteral writes a literal into the operation stream
func (w *Writer) writeLiteral(l lit) error {
	var err error
	state, state2, _ := w.State.states()
	if err = w.State.isMatch[state2].Encode(w.re, 0); err != nil {
		return err
	}
	litState := w.State.litState()
	match := w.State.dict.byteAt(int64(w.State.rep[0]) + 1)
	err = w.State.litCodec.Encode(w.re, l.b, state, match, litState)
	if err != nil {
		return err
	}
	w.State.updateStateLiteral()
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
func (w *Writer) writeMatch(m match) error {
	var err error
	if !(minDistance <= m.distance && m.distance <= maxDistance) {
		return errors.New("distance out of range")
	}
	dist := uint32(m.distance - minDistance)
	if !(MinLength <= m.n && m.n <= MaxLength) &&
		!(dist == w.State.rep[0] && m.n == 1) {
		return errors.New("length out of range")
	}
	state, state2, posState := w.State.states()
	if err = w.State.isMatch[state2].Encode(w.re, 1); err != nil {
		return err
	}
	var g int
	for g = 0; g < 4; g++ {
		if w.State.rep[g] == dist {
			break
		}
	}
	b := iverson(g < 4)
	if err = w.State.isRep[state].Encode(w.re, b); err != nil {
		return err
	}
	n := uint32(m.n - MinLength)
	if b == 0 {
		// simple match
		w.State.rep[3], w.State.rep[2], w.State.rep[1], w.State.rep[0] = w.State.rep[2],
			w.State.rep[1], w.State.rep[0], dist
		w.State.updateStateMatch()
		if err = w.State.lenCodec.Encode(w.re, n, posState); err != nil {
			return err
		}
		return w.State.distCodec.Encode(w.re, dist, n)
	}
	b = iverson(g != 0)
	if err = w.State.isRepG0[state].Encode(w.re, b); err != nil {
		return err
	}
	if b == 0 {
		// g == 0
		b = iverson(m.n != 1)
		if err = w.State.isRepG0Long[state2].Encode(w.re, b); err != nil {
			return err
		}
		if b == 0 {
			w.State.updateStateShortRep()
			return nil
		}
	} else {
		// g in {1,2,3}
		b = iverson(g != 1)
		if err = w.State.isRepG1[state].Encode(w.re, b); err != nil {
			return err
		}
		if b == 1 {
			// g in {2,3}
			b = iverson(g != 2)
			err = w.State.isRepG2[state].Encode(w.re, b)
			if err != nil {
				return err
			}
			if b == 1 {
				w.State.rep[3] = w.State.rep[2]
			}
			w.State.rep[2] = w.State.rep[1]
		}
		w.State.rep[1] = w.State.rep[0]
		w.State.rep[0] = dist
	}
	w.State.updateStateRep()
	return w.State.repLenCodec.Encode(w.re, n, posState)
}

// writeOp writes an operation value into the stream. It checks whether there
// is still enough space available using an upper limit for the size required.
func (w *Writer) writeOp(op operation) error {
	var err error
	switch x := op.(type) {
	case match:
		err = w.writeMatch(x)
	case lit:
		err = w.writeLiteral(x)
	}
	if err != nil {
		return err
	}
	return w.discard(op)
}

// discard processes an operation after it has been written into the
// compressed LZMA street by moving the dictionary head forward.
func (w *Writer) discard(op operation) error {
	k := op.Len()
	n, err := w.State.dict.(*hashDict).move(k)
	if err != nil {
		return fmt.Errorf("operation %s: move %d error %s", op, k, err)
	}
	if n < k {
		return fmt.Errorf("operation %s: move %d incomplete", op, k)
	}
	return nil
}

// compress does the actual compression. If all is set all data
// available will be compressed.
func (w *Writer) compress(all bool) error {
	ops, err := w.OpFinder.findOps(w.State, all)
	if err != nil {
		return err
	}
	for _, op := range ops {
		if err = w.writeOp(op); err != nil {
			return err
		}
	}
	w.State.dict.(*hashDict).sync()
	return nil
}

// errWriterClosed indicates that a writer has been closed once before.
var errWriterClosed = errors.New("writer is closed")

// Write puts the provided data into the writer.
func (w *Writer) Write(p []byte) (n int, err error) {
	if w.closed {
		return 0, errWriterClosed
	}
	for len(p) > 0 {
		var k int
		k, err = w.buf.Write(p)
		n += k
		if err != errLimit {
			return
		}
		p = p[k:]
		if err = w.compress(false); err != nil {
			return
		}
	}
	return
}

// This operation will be encoded to indicate that the stream has ended.
var eosMatch = match{distance: maxDistance, n: MinLength}

func (w *Writer) Close() (err error) {
	if w.closed {
		return errWriterClosed
	}
	w.closed = true
	if err = w.compress(true); err != nil {
		return err
	}
	if w.EOS {
		if err = w.writeMatch(eosMatch); err != nil {
			return err
		}
	}
	if err = w.re.Close(); err != nil {
		return err
	}
	return nil
}
