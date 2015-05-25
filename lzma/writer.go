package lzma

import (
	"errors"
	"io"

	"github.com/uli-go/xz/lzb"
)

// Writer supports the LZMA compression of a file.
//
// Using an arithmetic coder it cannot support flushing. A writer must be
// closed.
type Writer struct {
	lzb.Writer
	params *Parameters
}

// NewWriter creates a new writer. It writes the LZMA header. It will use the
// Default Parameters.
//
// Don't forget to call Close() for the writer after all data has been written.
//
// For high performance use a buffered writer. But be aware that Close will not
// flush it.
func NewWriter(w io.Writer) (lw *Writer, err error) {
	return NewWriterP(w, Default)
}

// NewWriterP creates a new writer with the given Parameters. It writes the
// LZMA header.
//
// Don't forget to call Close() for the writer after all data has been written.
//
// For high performance use a buffered writer. But be aware that Close will not
// flush it.
func NewWriterP(w io.Writer, p Parameters) (lw *Writer, err error) {
	if w == nil {
		return nil, errors.New("writer argument w is nil")
	}
	normalizeSizes(&p)
	if err = verifyParameters(&p); err != nil {
		return nil, err
	}
	if p.Size == 0 && !p.SizeInHeader {
		p.EOS = true
	}
	if err = writeHeader(w, &p); err != nil {
		return nil, err
	}
	panic("TODO")
}

// Parametes returns a copy of the parameters for the writer.
func (lw *Writer) Parameters() Parameters {
	return *lw.params
}

// Close closes the writer.
//
// Please note that the underlying writer will neither be flushed nor closed.
func (lw *Writer) Close() error {
	// function is necessary to have it explicitly documented.
	return lw.Writer.Close()
}
