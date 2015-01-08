package lzma

import (
	"io"
)

// defaultProperties defines the default properties for the Writer.
var defaultProperties = Properties{
	LC:      3,
	LP:      0,
	PB:      2,
	DictLen: 1 << 12}

// Writer supports the LZMA compression of a file. It cannot support flushing
// because of the arithmethic coder.
type Writer struct {
	w          io.Writer
	ow         *opWriter
	properties Properties
	unpackLen  uint64
	writtenLen uint64
	// end-of-stream marker required
	eos  bool
	dict *writerDict
	// hash table for four-byte sequences
	t4 *hashTable
}

// NewWriter creates a new LZMA writer using the given properties. It doesn't
// provide an unpack length and creates an explicit end of stream. The classic
// LZMA header will be created. If p is nil default parameters will be used.
func NewWriter(w io.Writer, p *Properties) (*Writer, error) {
	return NewWriterLenEOS(w, p, NoUnpackLen, true)
}

// NewWriterLen creates a new LZMA writer and a predefined length. There will
// be no end-of-stream marker created unless NoUnpackLen is used as length. If
// p is nil default parameters will be used.
func NewWriterLen(w io.Writer, p *Properties, length uint64) (*Writer, error) {
	return NewWriterLenEOS(w, p, length, false)
}

// newWriter creates a new writer without writing the header.
func newWriter(w io.Writer, p *Properties, length uint64, eos bool) (*Writer,
	error) {
	if length == NoUnpackLen {
		eos = true
	}
	if w == nil {
		return nil, newError("can't support a nil writer")
	}
	if p == nil {
		p = &defaultProperties
	}
	var err error
	if err = verifyProperties(p); err != nil {
		return nil, err
	}
	exp := hashTableExponent(p.DictLen)
	lw := &Writer{
		w:          w,
		properties: *p,
		unpackLen:  length,
		eos:        eos,
	}
	lw.dict, err = newWriterDict(int(p.DictLen), defaultBufferLen)
	if err != nil {
		return nil, err
	}
	lw.ow, err = newOpWriter(w, &lw.properties, lw.dict)
	if err != nil {
		return nil, err
	}
	lw.t4, err = newHashTable(exp, 4)
	if err != nil {
		return nil, err
	}
	return lw, nil
}

// putUint64LE puts the uint64 value into the byte slice as little endian
// value. The byte slice b must have at least place for 8 bytes.
func putUint64LE(b []byte, x uint64) {
	b[0] = byte(x)
	b[1] = byte(x >> 8)
	b[2] = byte(x >> 16)
	b[3] = byte(x >> 24)
	b[4] = byte(x >> 32)
	b[5] = byte(x >> 40)
	b[6] = byte(x >> 48)
	b[7] = byte(x >> 56)
}

// writeHeader writes the classic header into the output writer.
func (lw *Writer) writeHeader() error {
	err := writeProperties(lw.w, &lw.properties)
	if err != nil {
		return err
	}
	b := make([]byte, 8)
	putUint64LE(b, lw.unpackLen)
	_, err = lw.w.Write(b)
	return err
}

// NewWriterLenEOS creates a new LZMA writer. A predefinied length can be
// provided and the writing of an end-of-stream marker can be controlled. If
// the argument NoUnpackLen will be provided for the lenght a end-of-stream
// marker will be written regardless of the eos parameter.
func NewWriterLenEOS(w io.Writer, p *Properties, length uint64, eos bool) (*Writer, error) {
	lw, err := newWriter(w, p, length, eos)
	if err != nil {
		return nil, err
	}
	if err = lw.writeHeader(); err != nil {
		return nil, err
	}
	return lw, nil
}

// Write moves data into the internal buffer and triggers its compression.
func (l *Writer) Write(p []byte) (n int, err error) {
	for n < len(p) {
		k, err := l.dict.Write(p[n:])
		n += k
		if err != nil && err != errAgain {
			return n, err
		}
		if err = l.process(0); err != nil {
			return n, err
		}
	}
	return n, nil
}

// Close terminates the LZMA stream. It doesn't close the underlying writer
// though.
func (lw *Writer) Close() error {
	var err error
	if err = lw.process(allData); err != nil {
		return err
	}
	if lw.eos {
		if err = lw.ow.writeEOS(); err != nil {
			return err
		}
	}
	return lw.ow.Close()
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
	for i := 1; i < 3; i++ {
		// distance -1, -2, -3
		off := head - int64(i)
		if start <= off {
			offs = append(offs, off)
		}
	}
	if len(p) == 4 {
		// distances from the hash table
		offs = append(offs, lw.t4.Offsets(p)...)
	}
	for i := 3; i >= 0; i++ {
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

// readOp converts the head of the dictionary into a buffer. The head of the
// dictionary will be advanced by copying the data coverd by the operation into
// the hash table.
func (lw *Writer) readOp() (op operation, err error) {
	op, err = lw.findOp()
	if err != nil {
		return nil, err
	}
	n, err := lw.dict.Copy(lw.t4, op.Len())
	if err != nil {
		return nil, err
	}
	if n < op.Len() {
		return nil, errAgain
	}
	return op, nil
}

// process encodes the data written into the dictionary buffer. The allData
// flag requires all data remaining in the buffer to be encoded.
func (lw *Writer) process(flags int) error {
	var lowMark int
	if flags&allData == 0 {
		lowMark = maxLength
	}
	for lw.dict.Readable() >= lowMark {
		op, err := lw.readOp()
		if err != nil {
			return err
		}
		if err = lw.ow.WriteOp(op); err != nil {
			return err
		}
	}
	return nil
}
