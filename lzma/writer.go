package lzma

import (
	"io"

	"github.com/uli-go/xz/hash"
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
	properties Properties
	unpackLen  uint64
	writtenLen uint64
	// end-of-stream marker required
	eos  bool
	dict *writerDict
	// hash table for four-byte sequences
	t4 *hashTable
	// hash table for two-byte sequences
	t2 *hashTable
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

// hashTableExponent derives the hash table exponent from the dict length.
func hashTableExponent(dictLen uint32) int {
	e := 30 - nlz32(dictLen)
	switch {
	case e < minTableExponent:
		e = minTableExponent
	case e > maxTableExponent:
		e = maxTableExponent
	}
	return e
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
	dict, err := newWriterDict(defaultBufferLen, int(p.DictLen))
	if err != nil {
		return nil, err
	}
	exp := hashTableExponent(p.DictLen)
	lw := &Writer{
		w:          w,
		properties: *p,
		unpackLen:  length,
		eos:        eos,
		dict:       dict,
		t4:         newHashTable(exp, hash.NewCyclicPoly(4)),
		t2:         newHashTable(exp, hash.NewCyclicPoly(2)),
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
func writeHeader(w *Writer) error {
	err := writeProperties(w.w, &w.properties)
	if err != nil {
		return err
	}
	b := make([]byte, 8)
	putUint64LE(b, w.unpackLen)
	_, err = w.w.Write(b)
	return err
}

// NewWriterLenEOS creates a new LZMA writer. A predefinied length can be
// provided and the writing of an end-of-stream marker can be controlled. If
// the argument NoUnpackLen will be provided for the lenght a end-of-stream
// marker will be written regardless of the eos parameter.T
func NewWriterLenEOS(w io.Writer, p *Properties, length uint64, eos bool) (*Writer, error) {
	lw, err := newWriter(w, p, length, eos)
	if err != nil {
		return nil, err
	}
	if err = writeHeader(lw); err != nil {
		return nil, err
	}
	return lw, nil
}

// Writes data into the writer buffer.
func (l *Writer) Write(p []byte) (int, error) {
	panic("TODO")
}

// Close flushes all data out and writes the EOS marker if requested.
func (l *Writer) Close() error {
	panic("TODO")
}
