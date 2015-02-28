package lzma

import "io"

// Writer supports the LZMA compression of a file.
//
// Using an arithmetic coder it cannot support flushing. A writer must be
// closed.
type Writer struct {
	opCodec
	w      io.Writer
	re     *rangeEncoder
	params Parameters
	dict   *writerDict
	// hash table for four-byte sequences
	t4 *hashTable
}

// newDataWriter creates a writer that doesn't write a header for the stream.
func newDataWriter(w io.Writer, p *Parameters) (*Writer, error) {
	if w == nil {
		return nil, newError("can't support a nil writer")
	}
	lw := &Writer{
		w:      w,
		params: *p,
	}
	p = &lw.params
	normalizeSizes(p)
	var err error
	if err = verifyParameters(p); err != nil {
		return nil, err
	}
	if p.Size == 0 && !p.SizeInHeader {
		p.EOS = true
	}
	lw.dict, err = newWriterDict(int(p.DictSize), p.BufferSize)
	if err != nil {
		return nil, err
	}
	lw.re = newRangeEncoder(w)
	lw.opCodec.init(p.Properties(), lw.dict)
	if lw.t4, err = newHashTable(int(p.DictSize), 4); err != nil {
		return nil, err
	}
	return lw, nil
}

// NewWriter creates a new writer. It writes the LZMA header. It will use the
// Default Parameters.
//
// Don't forget to call Close() for the writer after all data has been written.
//
// For high performance use a buffered writer. But be aware that Close will not
// flush it.
func NewWriter(w io.Writer) (*Writer, error) {
	return NewWriterP(w, Default)
}

// NewWriterP creates a new writer with the given Parameters. It writes the
// LZMA header.
//
// Don't forget to call Close() for the writer after all data has been written.
func NewWriterP(w io.Writer, p Parameters) (*Writer, error) {
	lw, err := newDataWriter(w, &p)
	if err != nil {
		return nil, err
	}
	if err = writeHeader(lw.w, &lw.params); err != nil {
		return nil, err
	}
	return lw, nil
}

// Parameters returns the parameters of the LZMA writer.
func (lw *Writer) Parameters() Parameters {
	return lw.params
}

// Write moves data into the internal buffer and triggers its compression.
func (lw *Writer) Write(p []byte) (n int, err error) {
	end := lw.dict.end + int64(len(p))
	if end < 0 {
		panic("end counter overflow")
	}
	var rerr error
	if lw.params.SizeInHeader && end > lw.params.Size {
		p = p[:lw.params.Size-end]
		rerr = newError("write exceeds unpackLen")
	}
	for n < len(p) {
		k, err := lw.dict.Write(p[n:])
		n += k
		if err != nil && err != errAgain {
			return n, err
		}
		if err = lw.process(0); err != nil {
			return n, err
		}
	}
	return n, rerr
}

// Close terminates the LZMA stream. It doesn't close the underlying writer
// though and leaves it alone. In some scenarios explicit closing of the
// underlying writer is required.
func (lw *Writer) Close() error {
	var err error
	if err = lw.process(allData); err != nil {
		return err
	}
	if lw.params.EOS {
		if err = lw.writeEOS(); err != nil {
			return err
		}
	}
	if err = lw.re.Close(); err != nil {
		return err
	}
	return lw.dict.Close()
}

// The allData flag tells the process method that all data must be processed.
const allData = 1

// indicates an empty buffer
var errEmptyBuf = newError("empty buffer")

// potentialOffsets creates a list of potential offsets for matches.
func (lw *Writer) potentialOffsets(p []byte) []int64 {
	head := lw.dict.Offset()
	start := lw.dict.start
	offs := make([]int64, 0, 32)
	// add potential offsets with highest priority at the top
	for i := 1; i < 11; i++ {
		// distance 1 to 8
		off := head - int64(i)
		if start <= off {
			offs = append(offs, off)
		}
	}
	if len(p) == 4 {
		// distances from the hash table
		offs = append(offs, lw.t4.Offsets(p)...)
	}
	for i := 3; i >= 0; i-- {
		// distances from the repetition for length less than 4
		dist := int64(lw.rep[i]) + minDistance
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
func (lw *Writer) bestMatch(offsets []int64) (m match, err error) {
	// creates a match for 1
	head := lw.dict.Offset()
	off := int64(-1)
	length := 0
	for i := len(offsets) - 1; i >= 0; i-- {
		n := lw.dict.EqualBytes(head, offsets[i], maxLength)
		if n > length {
			off, length = offsets[i], n
		}
	}
	if off < 0 {
		err = errNoMatch
		return
	}
	if length == 1 {
		dist := int64(lw.rep[0]) + minDistance
		offRep0 := head - dist
		if off != offRep0 {
			err = errNoMatch
			return
		}
	}
	return match{distance: head - off, length: length}, nil
}

// findOp finds an operation for the head of the dictionary.
func (lw *Writer) findOp() (op operation, err error) {
	p := make([]byte, 4)
	n, err := lw.dict.PeekHead(p)
	if err != nil && err != errAgain && err != io.EOF {
		return nil, err
	}
	if n <= 0 {
		if n < 0 {
			panic("strange n")
		}
		return nil, errEmptyBuf
	}
	offs := lw.potentialOffsets(p[:n])
	m, err := lw.bestMatch(offs)
	if err == errNoMatch {
		return lit{b: p[0]}, nil
	}
	if err != nil {
		return nil, err
	}
	return m, nil
}

// discardOp advances the head of the dictionary and writes the the bytes into
// the hash table.
func (lw *Writer) discardOp(op operation) error {
	n, err := lw.dict.Copy(lw.t4, op.Len())
	if err != nil {
		return err
	}
	if n < op.Len() {
		return errAgain
	}
	return nil
}

// process encodes the data written into the dictionary buffer. The allData
// flag requires all data remaining in the buffer to be encoded.
func (lw *Writer) process(flags int) error {
	var lowMark int
	if flags&allData == 0 {
		lowMark = maxLength - 1
	}
	for lw.dict.Readable() > lowMark {
		op, err := lw.findOp()
		if err != nil {
			debug.Printf("findOp error %s\n", err)
			return err
		}
		if err = lw.writeOp(op); err != nil {
			return err
		}
		debug.Printf("op %s", op)
		if err = lw.discardOp(op); err != nil {
			return err
		}
	}
	return nil
}

// writeLiteral writes a literal into the operation stream
func (lw *Writer) writeLiteral(l lit) error {
	var err error
	state, state2, _ := lw.states()
	if err = lw.isMatch[state2].Encode(lw.re, 0); err != nil {
		return err
	}
	litState := lw.litState()
	match := lw.dict.Byte(int(lw.rep[0]) + 1)
	err = lw.litCodec.Encode(lw.re, l.b, state, match, litState)
	if err != nil {
		return err
	}
	lw.updateStateLiteral()
	return nil
}

// writeEOS writes the explicit EOS marker
func (lw *Writer) writeEOS() error {
	return lw.writeMatch(match{distance: maxDistance, length: minLength})
}

func iverson(ok bool) uint32 {
	if ok {
		return 1
	}
	return 0
}

// writeRep writes a repetition operation into the operation stream
func (lw *Writer) writeMatch(m match) error {
	var err error
	if !(minDistance <= m.distance && m.distance <= maxDistance) {
		return newError("distance out of range")
	}
	dist := uint32(m.distance - minDistance)
	if !(minLength <= m.length && m.length <= maxLength) &&
		!(dist == lw.rep[0] && m.length == 1) {
		return newError("length out of range")
	}
	state, state2, posState := lw.states()
	if err = lw.isMatch[state2].Encode(lw.re, 1); err != nil {
		return err
	}
	var g int
	for g = 0; g < 4; g++ {
		if lw.rep[g] == dist {
			break
		}
	}
	b := iverson(g < 4)
	if err = lw.isRep[state].Encode(lw.re, b); err != nil {
		return err
	}
	n := uint32(m.length - minLength)
	if b == 0 {
		// simple match
		lw.rep[3], lw.rep[2], lw.rep[1], lw.rep[0] = lw.rep[2],
			lw.rep[1], lw.rep[0], dist
		lw.updateStateMatch()
		if err = lw.lenCodec.Encode(lw.re, n, posState); err != nil {
			return err
		}
		return lw.distCodec.Encode(lw.re, dist, n)
	}
	b = iverson(g != 0)
	if err = lw.isRepG0[state].Encode(lw.re, b); err != nil {
		return err
	}
	if b == 0 {
		// g == 0
		b = iverson(m.length != 1)
		if err = lw.isRepG0Long[state2].Encode(lw.re, b); err != nil {
			return err
		}
		if b == 0 {
			lw.updateStateShortRep()
			return nil
		}
	} else {
		// g in {1,2,3}
		b = iverson(g != 1)
		if err = lw.isRepG1[state].Encode(lw.re, b); err != nil {
			return err
		}
		if b == 1 {
			// g in {2,3}
			b = iverson(g != 2)
			err = lw.isRepG2[state].Encode(lw.re, b)
			if err != nil {
				return err
			}
			if b == 1 {
				lw.rep[3] = lw.rep[2]
			}
			lw.rep[2] = lw.rep[1]
		}
		lw.rep[1] = lw.rep[0]
		lw.rep[0] = dist
	}
	lw.updateStateRep()
	return lw.repLenCodec.Encode(lw.re, n, posState)
}

// writeOp writes an operation value into the stream.
func (lw *Writer) writeOp(op operation) error {
	switch x := op.(type) {
	case match:
		return lw.writeMatch(x)
	case lit:
		return lw.writeLiteral(x)
	}
	panic("unknown operation type")
}
