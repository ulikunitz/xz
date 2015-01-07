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
	lw.dict = new(writerDict)
	err = initWriterDict(lw.dict, defaultBufferLen, int(p.DictLen))
	if err != nil {
		return nil, err
	}
	lw.ow, err = newOpWriter(w, &lw.properties, &lw.dict.dictionary)
	if err != nil {
		return nil, err
	}
	lw.t4, err = newHashTable(exp, 4)
	if err != nil {
		return nil, err
	}
	lw.t2, err = newHashTable(exp, 2)
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
// marker will be written regardless of the eos parameter.
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
func (l *Writer) Close() error {
	var err error
	if err = l.process(allData); err != nil {
		return err
	}
	if l.eos {
		if err = l.ow.writeEOS(); err != nil {
			return err
		}
	}
	return l.ow.Close()
}

const (
	allData = 1 << iota
)

// process encodes the data written into the dictionary buffer. The allData
// flag requires all data remaining in the buffer to be encoded.
func (l *Writer) process(flags int) error {
	lowMark := 0
	if flags&allData == 0 {
		lowMark = maxLength
	}
	for l.dict.buffered() >= lowMark {
		// transform head into operation
		// write operation
		// advance total pointer including updating hashes
		panic("TODO")
	}
	return nil
}
