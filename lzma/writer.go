package lzma

import (
	"io"

	"github.com/uli-go/xz/xlog"
)

// Writer supports the LZMA compression of a file.
//
// Using an arithmetic coder it cannot support flushing. A writer must be
// closed.
type Writer struct {
	w     io.Writer
	ow    *opWriter
	props Properties
	dict  *writerDict
	// hash table for four-byte sequences
	t4 *hashTable
}

// Default defines the properties used by NewWriter.
var Default = Properties{
	LC:      3,
	LP:      0,
	PB:      2,
	DictLen: 1 << 12}

// NewWriter creates a new writer. It writes the LZMA header. It will use the
// Default Properties.
//
// Don't forget to call Close() for the writer after all data has been written.
//
// For high performance use a buffered writer.
func NewWriter(w io.Writer) (*Writer, error) {
	return NewWriterP(w, Default)
}

// NewWriterP creates a new writer with the given Properties. It writes the
// LZMA header.
//
// Don't forget to call Close() for the writer after all data has been written.
func NewWriterP(w io.Writer, p Properties) (*Writer, error) {
	if w == nil {
		return nil, newError("can't support a nil writer")
	}
	var err error
	if err = verifyProperties(&p); err != nil {
		return nil, err
	}
	if p.Len == 0 && !p.LenInHeader {
		p.EOS = true
	}
	lw := &Writer{
		w:     w,
		props: p,
	}
	lw.dict, err = newWriterDict(int(p.DictLen), defaultBufferLen)
	if err != nil {
		return nil, err
	}
	lw.ow, err = newOpWriter(w, &p, lw.dict)
	if err != nil {
		return nil, err
	}
	lw.t4, err = newHashTable(int(p.DictLen), 4)
	if err != nil {
		return nil, err
	}
	if err = lw.writeHeader(); err != nil {
		return nil, err
	}
	return lw, nil
}

// Properties returns the properties of the LZMA writer.
func (lw *Writer) Properties() Properties {
	return lw.props
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
	err := writeProperties(lw.w, &lw.props)
	if err != nil {
		return err
	}
	b := make([]byte, 8)
	var l uint64
	if lw.props.LenInHeader {
		l = uint64(lw.props.Len)
	} else {
		l = noHeaderLen
	}
	putUint64LE(b, l)
	_, err = lw.w.Write(b)
	return err
}

// Write moves data into the internal buffer and triggers its compression.
func (lw *Writer) Write(p []byte) (n int, err error) {
	end := lw.dict.end + int64(len(p))
	if end < 0 {
		panic("end counter overflow")
	}
	var rerr error
	if lw.props.LenInHeader && end > lw.props.Len {
		p = p[:lw.props.Len-end]
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
	if lw.props.EOS {
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
			xlog.Printf(debug, "findOp error %s\n", err)
			return err
		}
		if err = lw.ow.WriteOp(op); err != nil {
			return err
		}
		xlog.Printf(debug, "op %s", op)
		if err = lw.discardOp(op); err != nil {
			return err
		}
	}
	return nil
}
