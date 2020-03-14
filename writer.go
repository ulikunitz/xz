package xz

import (
	"errors"
	"hash"
	"io"

	"github.com/ulikunitz/xz/filter"
	"github.com/ulikunitz/xz/lzma"
	"github.com/ulikunitz/xz/xzinternals"
)

// WriterConfig describe the parameters for an xz writer.
type WriterConfig struct {
	Properties *lzma.Properties
	DictCap    int
	BufSize    int
	BlockSize  int64
	// checksum method: CRC32, CRC64 or SHA256 (default: CRC64)
	CheckSum byte
	// Forces NoChecksum (default: false)
	NoCheckSum bool
	// match algorithm
	Matcher lzma.MatchAlgorithm
}

// fill replaces zero values with default values.
func (c *WriterConfig) fill() {
	if c.Properties == nil {
		c.Properties = &lzma.Properties{LC: 3, LP: 0, PB: 2}
	}
	if c.DictCap == 0 {
		c.DictCap = 8 * 1024 * 1024
	}
	if c.BufSize == 0 {
		c.BufSize = 4096
	}
	if c.BlockSize == 0 {
		c.BlockSize = maxInt64
	}
	if c.CheckSum == 0 {
		c.CheckSum = xzinternals.CRC64
	}
	if c.NoCheckSum {
		c.CheckSum = xzinternals.None
	}
}

// Verify checks the configuration for errors. Zero values will be
// replaced by default values.
func (c *WriterConfig) Verify() error {
	if c == nil {
		return errors.New("xz: writer configuration is nil")
	}
	c.fill()
	lc := lzma.Writer2Config{
		Properties: c.Properties,
		DictCap:    c.DictCap,
		BufSize:    c.BufSize,
		Matcher:    c.Matcher,
	}
	if err := lc.Verify(); err != nil {
		return err
	}
	if c.BlockSize <= 0 {
		return errors.New("xz: block size out of range")
	}
	if err := xzinternals.VerifyFlags(c.CheckSum); err != nil {
		return err
	}
	return nil
}

// newBlockWriter creates a new block writer.
func (c *WriterConfig) newBlockWriter(xz io.Writer, hash hash.Hash) (bw *xzinternals.BlockWriter, err error) {
	bw = &xzinternals.BlockWriter{
		CXZ:       xzinternals.NewCountingWriter(xz),
		BlockSize: c.BlockSize,
		Filters:   c.filters(),
		Hash:      hash,
	}

	fwc := &filter.WriterConfig{
		Properties: c.Properties,
		DictCap:    c.DictCap,
		BufSize:    c.BufSize,
		Matcher:    c.Matcher,
	}

	bw.W, err = filter.NewFilterWriteCloser(fwc, &bw.CXZ, bw.Filters)
	if err != nil {
		return nil, err
	}
	if bw.Hash.Size() != 0 {
		bw.MW = io.MultiWriter(bw.W, bw.Hash)
	} else {
		bw.MW = bw.W
	}
	return bw, nil
}

// filters creates the filter list for the given parameters.
func (c *WriterConfig) filters() []filter.Filter {
	return []filter.Filter{filter.NewLZMAFilter(int64(c.DictCap))}
}

// maxInt64 defines the maximum 64-bit signed integer.
const maxInt64 = 1<<63 - 1

// Writer compresses data written to it. It is an io.WriteCloser.
type Writer struct {
	WriterConfig

	xz      io.Writer
	bw      *xzinternals.BlockWriter
	newHash func() hash.Hash
	h       xzinternals.Header
	index   []xzinternals.Record
	closed  bool
}

// newBlockWriter creates a new block writer writes the header out.
func (w *Writer) newBlockWriter() error {
	var err error
	w.bw, err = w.WriterConfig.newBlockWriter(w.xz, w.newHash())
	if err != nil {
		return err
	}
	if err = w.bw.WriteHeader(w.xz); err != nil {
		return err
	}
	return nil
}

// closeBlockWriter closes a block writer and records the sizes in the
// index.
func (w *Writer) closeBlockWriter() error {
	var err error
	if err = w.bw.Close(); err != nil {
		return err
	}
	w.index = append(w.index, w.bw.Record())
	return nil
}

// NewWriter creates a new xz writer using default parameters.
func NewWriter(xz io.Writer) (w *Writer, err error) {
	return WriterConfig{}.NewWriter(xz)
}

// NewWriter creates a new Writer using the given configuration parameters.
func (c WriterConfig) NewWriter(xz io.Writer) (w *Writer, err error) {
	if err = c.Verify(); err != nil {
		return nil, err
	}
	w = &Writer{
		WriterConfig: c,
		xz:           xz,
		h:            xzinternals.Header{c.CheckSum},
		index:        make([]xzinternals.Record, 0, 4),
	}
	if w.newHash, err = xzinternals.NewHashFunc(c.CheckSum); err != nil {
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

// Write compresses the uncompressed data provided.
func (w *Writer) Write(p []byte) (n int, err error) {
	if w.closed {
		return 0, xzinternals.ErrClosed
	}
	for {
		k, err := w.bw.Write(p[n:])
		n += k
		if err != xzinternals.ErrNoSpace {
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

// Close closes the writer and adds the footer to the Writer. Close
// doesn't close the underlying writer.
func (w *Writer) Close() error {
	if w.closed {
		return xzinternals.ErrClosed
	}
	w.closed = true
	var err error
	if err = w.closeBlockWriter(); err != nil {
		return err
	}

	f := xzinternals.Footer{Flags: w.h.Flags}
	if f.IndexSize, err = xzinternals.WriteIndex(w.xz, w.index); err != nil {
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
