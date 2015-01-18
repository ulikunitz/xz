package lzma

import "io"

// Writer supports the LZMA compression of a file.
//
// Using an arithmetic coder it cannot support flushing. A writer must be
// closed.
type Writer struct {
	w      io.Writer
	ow     *opWriter
	params Parameters
	dict   *writerDict
	// hash table for four-byte sequences
	t4 *hashTable
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
	if w == nil {
		return nil, newError("can't support a nil writer")
	}
	var err error
	if err = verifyParameters(&p); err != nil {
		return nil, err
	}
	if p.Size == 0 && !p.SizeInHeader {
		p.EOS = true
	}
	lw := &Writer{
		w:      w,
		params: p,
	}
	lw.dict, err = newWriterDict(int(p.DictSize), p.BufferSize)
	if err != nil {
		return nil, err
	}
	lw.ow, err = newOpWriter(w, &p, lw.dict)
	if err != nil {
		return nil, err
	}
	lw.t4, err = newHashTable(int(p.DictSize), 4)
	if err != nil {
		return nil, err
	}
	if err = writeHeader(w, &lw.params); err != nil {
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
		if err = lw.ow.writeEOS(); err != nil {
			return err
		}
	}
	if err = lw.ow.Close(); err != nil {
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
		dist := int64(lw.ow.rep[i]) + minDistance
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
		dist := int64(lw.ow.rep[0]) + minDistance
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
		if err = lw.ow.WriteOp(op); err != nil {
			return err
		}
		debug.Printf("op %s", op)
		if err = lw.discardOp(op); err != nil {
			return err
		}
	}
	return nil
}
