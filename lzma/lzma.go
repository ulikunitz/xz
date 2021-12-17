package lzma

import (
	"errors"
	"io"
)

const (
	// mb give the number of bytes in a megabyte.
	mb = 1 << 20
)

// minDictSize defines the minumum supported dictionary size.
const minDictSize = 1 << 12

// ErrUnexpectedEOS reports an unexpected end-of-stream marker
var ErrUnexpectedEOS = errors.New("lzma: unexpected end of stream")

// ErrEncoding reports an encoding error
var ErrEncoding = errors.New("lzma: wrong encoding")

// NewReader creates a reader for LZMA-compressed streams. It reads the LZTMA
// header and creates a reader.
func NewReader(lzma io.Reader) (r io.Reader, err error) {
	headerBuf := make([]byte, headerLen)
	if _, err = io.ReadFull(z, headerBuf); err != nil {
		if err == io.EOF {
			err = io.ErrUnexpectedEOF
		}
		return nil, err
	}

	var p params
	if err = p.UnmarshalBinary(headerBuf); err != nil {
		return nil, err
	}
	if p.dictSize < minDictSize {
		p.dictSize = minDictSize
	}
	if err = p.Verify(); err != nil {
		return nil, err
	}

	t := new(reader)
	if err = t.init(z, p); err != nil {
		return nil, err
	}

	return t, nil
}

// WriterConfig provides configuration parameters for the LZMA writer.
type WriterConfig struct {
	Properties
	// set to true if you want LC, LP and PB actuially zero
	PropertiesInitialized bool
	DictSize              int
	MemoryBudget          int
	Effort                int
}

// NewWriter creates a single-threaded writer of LZMA files.
func NewWriter(z io.Writer) (w io.WriteCloser, err error) {
	cfg := WriterConfig{
		Properties:            Properties{LC: 3, LP: 0, PB: 2},
		PropertiesInitialized: true,
		DictSize:              8 * mb,
		MemoryBudget:          10 * mb,
		Effort:                5,
	}
	return NewWriterConfig(z, cfg)
}

// NewWriterConfig creates a new writer generating LZMA files.
func NewWriterConfig(z io.Writer, cfg WriterConfig) (w io.WriteCloser, err error) {
	panic("TODO")
}
