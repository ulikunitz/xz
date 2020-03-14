// Package xz supports the compression and decompression of xz files. It
// supports version 1.0.4 of the specification without the non-LZMA2
// filters. See http://tukaani.org/xz/xz-file-format-1.0.4.txt
package xz

import (
	"errors"
	"io"

	"github.com/ulikunitz/xz/lzma"
	"github.com/ulikunitz/xz/xzinternals"
)

// ReaderConfig defines the parameters for the xz reader. The
// SingleStream parameter requests the reader to assume that the
// underlying stream contains only a single stream.
type ReaderConfig struct {
	DictCap      int
	SingleStream bool
}

// fill replaces all zero values with their default values.
func (c *ReaderConfig) fill() {
	if c.DictCap == 0 {
		c.DictCap = 8 * 1024 * 1024
	}
}

// Verify checks the reader parameters for Validity. Zero values will be
// replaced by default values.
func (c *ReaderConfig) Verify() error {
	if c == nil {
		return errors.New("xz: reader parameters are nil")
	}
	lc := lzma.Reader2Config{DictCap: c.DictCap}
	if err := lc.Verify(); err != nil {
		return err
	}
	return nil
}

// Reader supports the reading of one or multiple xz streams.
type Reader struct {
	ReaderConfig

	xz io.Reader
	sr *xzinternals.StreamReader
}

// NewReader creates a new xz reader using the default parameters.
// The function reads and checks the header of the first XZ stream. The
// reader will process multiple streams including padding.
func NewReader(xz io.Reader) (r *Reader, err error) {
	return ReaderConfig{}.NewReader(xz)
}

// NewReader creates an xz stream reader. The created reader will be
// able to process multiple streams and padding unless a SingleStream
// has been set in the reader configuration c.
func (c ReaderConfig) NewReader(xz io.Reader) (r *Reader, err error) {
	if err = c.Verify(); err != nil {
		return nil, err
	}
	r = &Reader{
		ReaderConfig: c,
		xz:           xz,
	}
	if r.sr, err = xzinternals.NewStreamReader(xz, c.DictCap); err != nil {
		if err == io.EOF {
			err = io.ErrUnexpectedEOF
		}
		return nil, err
	}
	return r, nil
}

var errUnexpectedData = errors.New("xz: unexpected data after stream")

// Read reads uncompressed data from the stream.
func (r *Reader) Read(p []byte) (n int, err error) {
	for n < len(p) {
		if r.sr == nil {
			if r.SingleStream {
				data := make([]byte, 1)
				_, err = io.ReadFull(r.xz, data)
				if err != io.EOF {
					return n, errUnexpectedData
				}
				return n, io.EOF
			}
			for {
				if err = r.ReaderConfig.Verify(); err != nil {
					break
				}

				r.sr, err = xzinternals.NewStreamReader(r.xz, r.ReaderConfig.DictCap)
				if err != xzinternals.ErrPadding {
					break
				}
			}
			if err != nil {
				return n, err
			}
		}
		k, err := r.sr.Read(p[n:])
		n += k
		if err != nil {
			if err == io.EOF {
				r.sr = nil
				continue
			}
			return n, err
		}
	}
	return n, nil
}
