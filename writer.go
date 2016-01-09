package xz

import (
	"errors"
	"hash"
	"io"

	"github.com/ulikunitz/xz/lzma2"
)

// WriterParams describe the parameters for a writer.
type WriterParams struct {
	lzma2.WriterParams
	BlockSize int64
	// checksum method: CRC32, CRC64 or SHA256
	CheckSum byte
}

// filters creates the filte list for the given parameters.
func (p *WriterParams) filters() []filter {
	return []filter{&lzmaFilter{int64(p.DictCap)}}
}

func (p *WriterParams) Verify() error {
	var err error
	if err = p.WriterParams.Verify(); err != nil {
		return err
	}
	if p.BlockSize <= 0 {
		return errors.New("xz: block size out of range")
	}
	if err = verifyFlags(p.CheckSum); err != nil {
		return err
	}
	return nil
}

// maxInt64 defines the maximum 64-bit signed integer.
const maxInt64 = 1<<63 - 1

// WriterDefaults defines the defaults for the Writer parameters.
var WriterDefaults = WriterParams{
	WriterParams: lzma2.WriterDefaults,
	BlockSize:    maxInt64,
	CheckSum:     CRC64,
}

// verifyFilters checks the filter list for the length and the right
// sequence of filters.
func verifyFilters(f []filter) error {
	if len(f) == 0 {
		return errors.New("xz: no filters")
	}
	if len(f) > 4 {
		return errors.New("xz: more than four filters")
	}
	for _, g := range f[:len(f)-1] {
		if g.last() {
			return errors.New("xz: last filter is not last")
		}
	}
	if !f[len(f)-1].last() {
		return errors.New("xz: wrong last filter")
	}
	return nil
}

// newFilterWriteCloser converts a filter list into a WriteCloser that
// can be used by a blockWriter.
func newFilterWriteCloser(w io.Writer, f []filter, p *WriterParams,
) (fw io.WriteCloser, err error) {
	if err = verifyFilters(f); err != nil {
		return nil, err
	}
	fw = nopWriteCloser(w)
	for i := len(f) - 1; i >= 0; i-- {
		fw, err = f[i].writeCloser(fw, p)
		if err != nil {
			return nil, err
		}
	}
	return fw, nil
}

// nopWCloser implements a WriteCloser with a Close method not doing
// anything.
type nopWCloser struct {
	io.Writer
}

// Close returns nil and doesn't do anything else.
func (c nopWCloser) Close() error {
	return nil
}

// nopWriteCloser converts the Writer into a WriteCloser with a Close
// function that does nothing beside returning nil.
func nopWriteCloser(w io.Writer) io.WriteCloser {
	return nopWCloser{w}
}

type Writer struct {
	WriterParams

	xz      io.Writer
	bw      *blockWriter
	newHash func() hash.Hash
	h       header
	index   []record
	closed  bool
}

func (w *Writer) newBlockWriter() error {
	var err error
	w.bw, err = newBlockWriter(w.xz, w.newHash(), &w.WriterParams)
	if err != nil {
		return err
	}
	if err = w.bw.writeHeader(w.xz); err != nil {
		return err
	}
	return nil
}

func (w *Writer) closeBlockWriter() error {
	var err error
	if err = w.bw.Close(); err != nil {
		return err
	}
	w.index = append(w.index, w.bw.record())
	return nil
}

func NewWriter(xz io.Writer) *Writer {
	w, err := NewWriterParams(xz, &WriterDefaults)
	if err != nil {
		panic(err)
	}
	return w
}

func NewWriterParams(xz io.Writer, p *WriterParams) (w *Writer, err error) {
	if err = p.Verify(); err != nil {
		return nil, err
	}
	w = &Writer{
		WriterParams: *p,
		xz:           xz,
		h:            header{p.CheckSum},
		index:        make([]record, 0, 4),
	}
	if w.newHash, err = newHashFunc(p.CheckSum); err != nil {
		return nil, err
	}
	data, err := w.h.MarshalBinary()
	if _, err = xz.Write(data); err != nil {
		return nil, err
	}
	if err = w.newBlockWriter(); err != nil {
		return nil, err
	}
	return w, nil

}

func (w *Writer) Write(p []byte) (n int, err error) {
	if w.closed {
		return 0, errClosed
	}
	for {
		k, err := w.bw.Write(p[n:])
		n += k
		if err != errNoSpace {
			return n, err
		}
		if err = w.closeBlockWriter(); err != nil {
			return n, err
		}
		if err = w.newBlockWriter(); err != nil {
			return n, err
		}
	}
}

func (w *Writer) Close() error {
	if w.closed {
		return errClosed
	}
	w.closed = true
	var err error
	if err = w.closeBlockWriter(); err != nil {
		return err
	}

	f := footer{flags: w.h.flags}
	if f.indexSize, err = writeIndex(w.xz, w.index); err != nil {
		return err
	}
	data, err := f.MarshalBinary()
	if err != nil {
		return err
	}
	if _, err = w.xz.Write(data); err != nil {
		return err
	}
	return nil
}

// cntWriter is a writer that counts all data written to it.
type cntWriter struct {
	w io.Writer
	n int64
}

func (cw *cntWriter) Write(p []byte) (n int, err error) {
	n, err = cw.w.Write(p)
	cw.n += int64(n)
	if err == nil && cw.n < 0 {
		return n, errors.New("xz: counter overflow")
	}
	return
}

type blockWriter struct {
	cxz       cntWriter
	mw        io.Writer
	w         io.WriteCloser
	n         int64
	blockSize int64
	closed    bool
	headerLen int

	filters []filter
	hash    hash.Hash
}

func newBlockWriter(xz io.Writer, hash hash.Hash, p *WriterParams,
) (bw *blockWriter, err error) {
	bw = &blockWriter{
		cxz:       cntWriter{w: xz},
		blockSize: p.BlockSize,
		filters:   p.filters(),
		hash:      hash,
	}
	bw.w, err = newFilterWriteCloser(&bw.cxz, bw.filters, p)
	if err != nil {
		return nil, err
	}
	bw.mw = io.MultiWriter(bw.w, bw.hash)
	return bw, nil
}

func (bw *blockWriter) writeHeader(w io.Writer) error {
	h := blockHeader{
		compressedSize:   -1,
		uncompressedSize: -1,
		filters:          bw.filters,
	}
	if bw.closed {
		h.compressedSize = bw.compressedSize()
		h.uncompressedSize = bw.uncompressedSize()
	}
	data, err := h.MarshalBinary()
	if err != nil {
		return err
	}
	if _, err = w.Write(data); err != nil {
		return err
	}
	bw.headerLen = len(data)
	return nil
}

func (bw *blockWriter) compressedSize() int64 {
	return bw.cxz.n
}

func (bw *blockWriter) uncompressedSize() int64 {
	return bw.n
}

func (bw *blockWriter) unpaddedSize() int64 {
	if bw.headerLen <= 0 {
		panic("xz: block header not written")
	}
	n := int64(bw.headerLen)
	n += bw.compressedSize()
	n += int64(bw.hash.Size())
	return n
}

func (bw *blockWriter) record() record {
	return record{bw.unpaddedSize(), bw.uncompressedSize()}
}

var errClosed = errors.New("xz: writer already closed")

var errNoSpace = errors.New("xz: no space")

func (bw *blockWriter) Write(p []byte) (n int, err error) {
	if bw.closed {
		return 0, errClosed
	}

	t := bw.blockSize - bw.n
	if int64(len(p)) > t {
		err = errNoSpace
		p = p[:t]
	}

	var werr error
	n, werr = bw.mw.Write(p)
	bw.n += int64(n)
	if werr != nil {
		return n, werr
	}
	return n, err
}

func (bw *blockWriter) Close() error {
	if bw.closed {
		return errClosed
	}
	bw.closed = true
	if err := bw.w.Close(); err != nil {
		return err
	}
	s := bw.hash.Size()
	k := padLen(bw.cxz.n)
	p := make([]byte, k+s)
	bw.hash.Sum(p[k:k])
	if _, err := bw.cxz.w.Write(p); err != nil {
		return err
	}
	return nil
}
