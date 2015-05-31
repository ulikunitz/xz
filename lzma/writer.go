package lzma

import (
	"errors"
	"io"

	"github.com/uli-go/xz/lzb"
)

// NewWriter creates a new writer. It writes the LZMA header. It will use the
// Default Parameters.
//
// Don't forget to call Close() for the writer after all data has been written.
//
// For high performance use a buffered writer. But be aware that Close will not
// flush it.
func NewWriter(w io.Writer) (lw io.WriteCloser, err error) {
	return NewWriterP(w, Default)
}

// NewWriterP creates a new writer with the given Parameters. It writes the
// LZMA header.
//
// Don't forget to call Close() for the writer after all data has been written.
//
// For high performance use a buffered writer. But be aware that Close will not
// flush it.
func NewWriterP(w io.Writer, p Parameters) (lw io.WriteCloser, err error) {
	if w == nil {
		return nil, errors.New("writer argument w is nil")
	}
	q := lzbParameters(&p)
	q.NormalizeWriterSizes()
	if err = q.Verify(); err != nil {
		return nil, err
	}
	if !q.SizeInHeader {
		q.EOS = true
	}
	if err = writeHeader(w, &q); err != nil {
		return nil, err
	}
	lw, err = lzb.NewWriter(w, q)
	if q.SizeInHeader {
		lw = &lzb.LimitedWriteCloser{W: lw, N: q.Size}
	}
	return
}
