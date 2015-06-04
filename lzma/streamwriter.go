package lzma

import (
	"fmt"
	"io"
)

// OpFinder enables the support of multiple different OpFinder
// algorithms.
type OpFinder interface {
	findOps(s *State, all bool) []operation
	fmt.Stringer
}

// Writer produces an LZMA stream. EOS requests Close to write an
// end-of-stream marker.
type Writer struct {
	OpFinder OpFinder
	Params   Parameters
	state    *State
	re       *rangeEncoder
	buf      *buffer
	closed   bool
	start    int64
}

// NewStreamWriter creates a new writer instance.
func NewStreamWriter(pw io.Writer, p Parameters) (w *Writer, err error) {
	if err = p.Verify(); err != nil {
		return
	}
	if !p.SizeInHeader {
		p.EOS = true
	}
	buf, err := newBuffer(p.DictSize + p.ExtraBufSize)
	if err != nil {
		return nil, err
	}
	d, err := newHashDict(buf, buf.bottom, p.DictSize)
	if err != nil {
		return nil, err
	}
	d.sync()
	state := NewState(p.Properties(), d)
	w = &Writer{
		Params:   p,
		OpFinder: Greedy,
		state:    state,
		buf:      buf,
		re:       newRangeEncoder(pw),
		start:    buf.top,
	}
	return w, nil
}

// writeLiteral writes a literal into the operation stream
func (w *Writer) writeLiteral(l lit) error {
	var err error
	state, state2, _ := w.state.states()
	if err = w.state.isMatch[state2].Encode(w.re, 0); err != nil {
		return err
	}
	litState := w.state.litState()
	match := w.state.dict.byteAt(int64(w.state.rep[0]) + 1)
	err = w.state.litCodec.Encode(w.re, l.b, state, match, litState)
	if err != nil {
		return err
	}
	w.state.updateStateLiteral()
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
		panic(rangeError{"match distance", m.distance})
	}
	dist := uint32(m.distance - minDistance)
	if !(MinLength <= m.n && m.n <= MaxLength) &&
		!(dist == w.state.rep[0] && m.n == 1) {
		panic(rangeError{"match length", m.n})
	}
	state, state2, posState := w.state.states()
	if err = w.state.isMatch[state2].Encode(w.re, 1); err != nil {
		return err
	}
	g := 0
	for ; g < 4; g++ {
		if w.state.rep[g] == dist {
			break
		}
	}
	b := iverson(g < 4)
	if err = w.state.isRep[state].Encode(w.re, b); err != nil {
		return err
	}
	n := uint32(m.n - MinLength)
	if b == 0 {
		// simple match
		w.state.rep[3], w.state.rep[2], w.state.rep[1], w.state.rep[0] =
			w.state.rep[2], w.state.rep[1], w.state.rep[0], dist
		w.state.updateStateMatch()
		if err = w.state.lenCodec.Encode(w.re, n, posState); err != nil {
			return err
		}
		return w.state.distCodec.Encode(w.re, dist, n)
	}
	b = iverson(g != 0)
	if err = w.state.isRepG0[state].Encode(w.re, b); err != nil {
		return err
	}
	if b == 0 {
		// g == 0
		b = iverson(m.n != 1)
		if err = w.state.isRepG0Long[state2].Encode(w.re, b); err != nil {
			return err
		}
		if b == 0 {
			w.state.updateStateShortRep()
			return nil
		}
	} else {
		// g in {1,2,3}
		b = iverson(g != 1)
		if err = w.state.isRepG1[state].Encode(w.re, b); err != nil {
			return err
		}
		if b == 1 {
			// g in {2,3}
			b = iverson(g != 2)
			err = w.state.isRepG2[state].Encode(w.re, b)
			if err != nil {
				return err
			}
			if b == 1 {
				w.state.rep[3] = w.state.rep[2]
			}
			w.state.rep[2] = w.state.rep[1]
		}
		w.state.rep[1] = w.state.rep[0]
		w.state.rep[0] = dist
	}
	w.state.updateStateRep()
	return w.state.repLenCodec.Encode(w.re, n, posState)
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
	err = w.discard(op)
	return err
}

// discard processes an operation after it has been written into the
// compressed LZMA street by moving the dictionary head forward.
func (w *Writer) discard(op operation) error {
	k := op.Len()
	n, err := w.state.dict.(*hashDict).move(k)
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
	ops := w.OpFinder.findOps(w.state, all)
	for _, op := range ops {
		if err := w.writeOp(op); err != nil {
			return err
		}
	}
	w.state.dict.(*hashDict).sync()
	return nil
}

// Write puts the provided data into the writer.
func (w *Writer) Write(p []byte) (n int, err error) {
	if w.closed {
		return 0, errWriterClosed
	}
	if w.Params.SizeInHeader {
		r := w.start + w.Params.Size - w.buf.top
		if r <= 0 {
			return 0, errWriteLimit
		}
		if int64(len(p)) > r {
			p = p[0:r]
			err = errWriteLimit
		}
	}
	for len(p) > 0 {
		k, werr := w.buf.Write(p)
		n += k
		p = p[k:]
		if werr != nil {
			if werr != errWriteLimit {
				return n, werr
			}
			if werr = w.compress(false); werr != nil {
				return n, werr
			}
		}
	}
	return n, err
}

func (w *Writer) ReadFrom(r io.Reader) (n int64, err error) {
	if w.closed {
		return 0, errWriterClosed
	}
	for {
		if w.Params.SizeInHeader && w.start+w.Params.Size <= w.buf.top {
			return 0, errWriteLimit
		}
		p, err := w.buf.writeSlice(w.buf.writeLimit)
		if err != nil {
			if err != errAgain {
				return n, err
			}
			if err = w.compress(false); err != nil {
				return n, err
			}
			continue
		}
		k, err := r.Read(p)
		n += int64(k)
		w.buf.top += int64(k)
		if err != nil {
			if err != io.EOF {
				return n, err
			}
			return n, nil
		}
	}
}

func (w *Writer) WriteByte(c byte) error {
	if w.closed {
		return errWriterClosed
	}
	if w.Params.SizeInHeader && w.start+w.Params.Size <= w.buf.top {
		return errWriteLimit
	}
	var err error
	if err = w.buf.WriteByte(c); err == nil {
		return nil
	}
	if err = w.compress(false); err != nil {
		return err
	}
	if err = w.buf.WriteByte(c); err != nil {
		panic("no space for write")
	}
	return nil
}

// This operation will be encoded to indicate that the stream has ended.
var eosMatch = match{distance: maxDistance, n: MinLength}

func (w *Writer) Size() int64 { return w.buf.top - w.start }

// Close closes the writer.
func (w *Writer) Close() (err error) {
	if w.closed {
		return errWriterClosed
	}
	if w.Params.SizeInHeader {
		n := w.Size()
		if n > w.Params.Size {
			panic(fmt.Errorf("w.N=%d larger than requested size %d",
				n, w.Params.Size))
		}
		if n < w.Params.Size {
			return errEarlyClose
		}
	}
	if err = w.compress(true); err != nil {
		return err
	}
	if w.Params.EOS {
		if err = w.writeMatch(eosMatch); err != nil {
			return err
		}
	}
	if err = w.re.Close(); err != nil {
		return err
	}
	w.closed = true
	return nil
}
