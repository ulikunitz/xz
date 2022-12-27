// Copyright 2014-2022 Ulrich Kunitz. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package xz supports the compression and decompression of xz files. It
// supports version 1.1.0 of the specification without the non-LZMA2
// filters. See http://tukaani.org/xz/xz-file-format-1.1.0.txt
package xz

import (
	"bytes"
	"errors"
	"fmt"
	"hash"
	"io"
	"runtime"

	"github.com/ulikunitz/xz/lzma"
)

var errReaderClosed = errors.New("xz: reader closed")

// ReaderConfig defines the parameters for the xz reader. The SingleStream
// parameter requests the reader to assume that the underlying stream contains
// only a single stream without padding.
//
// The workers variable controls the number of parallel workers decoding the
// file. It only has an effect if the file was encoded in a way that it created
// blocks with the compressed size set in the headers. If Workers not 1 the
// Workers variable in LZMAConfig will be ignored.
type ReaderConfig struct {
	LZMA lzma.Reader2Config

	// input contains only a single stream without padding.
	SingleStream bool

	// Wokrkers defines the number of readers for parallel reading. The
	// default is the value of GOMAXPROCS.
	Workers int
}

// ApplyDefaults sets
func (cfg *ReaderConfig) ApplyDefaults() {
	cfg.LZMA.ApplyDefaults()
	if cfg.Workers == 0 {
		cfg.Workers = runtime.GOMAXPROCS(0)
	}
}

// Verify checks the reader parameters for Validity. Zero values will be
// replaced by default values.
func (cfg *ReaderConfig) Verify() error {
	if cfg == nil {
		return errors.New("xz: reader parameters are nil")
	}

	if err := cfg.LZMA.Verify(); err != nil {
		return err
	}
	return nil
}

// Reader supports the reading of one or multiple xz streams.
type Reader struct {
	cfg ReaderConfig

	xz io.Reader
	sr *streamReader
}

// streamReader decodes a single xz stream
type streamReader struct {
	ReaderConfig

	xz      io.Reader
	br      *blockReader
	newHash func() hash.Hash
	h       header
	index   []record
}

// NewReader creates a new xz reader using the default parameters.
// The function reads and checks the header of the first XZ stream. The
// reader will process multiple streams including padding.
func NewReader(xz io.Reader) (r *Reader, err error) {
	return ReaderConfig{}.newReader(xz)
}

// NewReaderConfig instantioates a new reader using a configuration parameter.
func NewReaderConfig(xz io.Reader, cfg ReaderConfig) (r *Reader, err error) {
	return cfg.newReader(xz)
}

// newReader creates an xz stream reader. The created reader will be
// able to process multiple streams and padding unless a SingleStream
// has been set in the reader configuration c.
func (cfg ReaderConfig) newReader(xz io.Reader) (r *Reader, err error) {
	cfg.ApplyDefaults()
	if err = cfg.Verify(); err != nil {
		return nil, err
	}
	r = &Reader{
		cfg: cfg,
		xz:  xz,
	}
	if r.sr, err = cfg.newStreamReader(xz); err != nil {
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
	if r.xz == nil {
		return 0, errReaderClosed
	}
	for n < len(p) {
		if r.sr == nil {
			if r.cfg.SingleStream {
				data := make([]byte, 1)
				_, err = io.ReadFull(r.xz, data)
				if err != io.EOF {
					return n, errUnexpectedData
				}
				return n, io.EOF
			}
			for {
				r.sr, err = r.cfg.newStreamReader(r.xz)
				if err != errPadding {
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

// Close closes the xz reader. The function must be called to clear Go routines
// that may be used by the LZMA2 reader.
func (r *Reader) Close() error {
	if r.xz == nil {
		return errReaderClosed
	}
	if r.sr != nil {
		err := r.sr.Close()
		if err != nil && err != errReaderClosed {
			return err
		}
		r.sr = nil
	}
	r.xz = nil
	return nil
}

var errPadding = errors.New("xz: padding (4 zero bytes) encountered")

// newStreamReader creates a new xz stream reader using the given configuration
// parameters. NewReader reads and checks the header of the xz stream.
func (cfg ReaderConfig) newStreamReader(xz io.Reader) (r *streamReader, err error) {
	if err = cfg.Verify(); err != nil {
		return nil, err
	}
	data := make([]byte, HeaderLen)
	if _, err := io.ReadFull(xz, data[:4]); err != nil {
		return nil, err
	}
	if bytes.Equal(data[:4], []byte{0, 0, 0, 0}) {
		return nil, errPadding
	}
	if _, err = io.ReadFull(xz, data[4:]); err != nil {
		if err == io.EOF {
			err = io.ErrUnexpectedEOF
		}
		return nil, err
	}
	r = &streamReader{
		ReaderConfig: cfg,
		xz:           xz,
		index:        make([]record, 0, 4),
	}
	if err = r.h.UnmarshalBinary(data); err != nil {
		return nil, err
	}
	if r.newHash, err = newHashFunc(r.h.flags); err != nil {
		return nil, err
	}
	return r, nil
}

// readTail reads the index body and the xz footer.
func (r *streamReader) readTail() error {
	index, n, err := readIndexBody(r.xz, len(r.index))
	if err != nil {
		if err == io.EOF {
			err = io.ErrUnexpectedEOF
		}
		return err
	}

	for i, rec := range r.index {
		if rec != index[i] {
			return fmt.Errorf("xz: record %d is %v; want %v",
				i, rec, index[i])
		}
	}

	p := make([]byte, footerLen)
	if _, err = io.ReadFull(r.xz, p); err != nil {
		if err == io.EOF {
			err = io.ErrUnexpectedEOF
		}
		return err
	}
	var f footer
	if err = f.UnmarshalBinary(p); err != nil {
		return err
	}
	if f.flags != r.h.flags {
		return errors.New("xz: footer flags incorrect")
	}
	if f.indexSize != int64(n)+1 {
		return errors.New("xz: index size in footer wrong")
	}
	return nil
}

// Read reads actual data from the xz stream.
func (r *streamReader) Read(p []byte) (n int, err error) {
	if r.xz == nil {
		return 0, errReaderClosed
	}
	for n < len(p) {
		if r.br == nil {
			bh, hlen, err := readBlockHeader(r.xz)
			if err != nil {
				if err == errIndexIndicator {
					if err = r.readTail(); err != nil {
						return n, err
					}
					return n, io.EOF
				}
				return n, err
			}
			r.br, err = r.ReaderConfig.newBlockReader(r.xz, bh,
				hlen, r.newHash())
			if err != nil {
				return n, err
			}
		}
		k, err := r.br.Read(p[n:])
		n += k
		if err != nil {
			if err == io.EOF {
				r.index = append(r.index, r.br.record())
				r.br = nil
			} else {
				return n, err
			}
		}
	}
	return n, nil
}

// Close closes the stream reader. It is required to clean up go routines in the
// LZMA2 reader implementation.
func (r *streamReader) Close() error {
	if r.xz == nil {
		return errReaderClosed
	}
	if r.br != nil {
		err := r.br.Close()
		if err != nil && err != errReaderClosed {
			return err
		}
		r.br = nil
	}
	r.xz = nil
	return nil
}

// countingReader is a reader that counts the bytes read.
type countingReader struct {
	r io.Reader
	n int64
}

// Read reads data from the wrapped reader and adds it to the n field.
func (lr *countingReader) Read(p []byte) (n int, err error) {
	n, err = lr.r.Read(p)
	lr.n += int64(n)
	return n, err
}

// blockReader supports the reading of a block.
type blockReader struct {
	lxz       countingReader
	header    *blockHeader
	headerLen int
	n         int64
	hash      hash.Hash
	r         io.Reader
	fr        io.ReadCloser
}

// newBlockReader creates a new block reader.
func (cfg *ReaderConfig) newBlockReader(xz io.Reader, h *blockHeader,
	hlen int, hash hash.Hash) (br *blockReader, err error) {

	br = &blockReader{
		lxz:       countingReader{r: xz},
		header:    h,
		headerLen: hlen,
		hash:      hash,
	}

	br.fr, err = cfg.newFilterReader(&br.lxz, h.filters)
	if err != nil {
		return nil, err
	}
	if br.hash.Size() != 0 {
		br.r = io.TeeReader(br.fr, br.hash)
	} else {
		br.r = br.fr
	}

	return br, nil
}

// uncompressedSize returns the uncompressed size of the block.
func (br *blockReader) uncompressedSize() int64 {
	return br.n
}

// compressedSize returns the compressed size of the block.
func (br *blockReader) compressedSize() int64 {
	return br.lxz.n
}

// unpaddedSize computes the unpadded size for the block.
func (br *blockReader) unpaddedSize() int64 {
	n := int64(br.headerLen)
	n += br.compressedSize()
	n += int64(br.hash.Size())
	return n
}

// record returns the index record for the current block.
func (br *blockReader) record() record {
	return record{br.unpaddedSize(), br.uncompressedSize()}
}

// Read reads data from the block.
func (br *blockReader) Read(p []byte) (n int, err error) {
	if br.fr == nil {
		return 0, errReaderClosed
	}
	n, err = br.r.Read(p)
	br.n += int64(n)

	u := br.header.uncompressedSize
	if u >= 0 && br.uncompressedSize() > u {
		return n, errors.New("xz: wrong uncompressed size for block")
	}
	c := br.header.compressedSize
	if c >= 0 && br.compressedSize() > c {
		return n, errors.New("xz: wrong compressed size for block")
	}
	if err != io.EOF {
		return n, err
	}
	if br.uncompressedSize() < u || br.compressedSize() < c {
		return n, io.ErrUnexpectedEOF
	}

	s := br.hash.Size()
	k := padLen(br.lxz.n)
	q := make([]byte, k+s, k+2*s)
	if _, err = io.ReadFull(br.lxz.r, q); err != nil {
		if err == io.EOF {
			err = io.ErrUnexpectedEOF
		}
		return n, err
	}
	if !allZeros(q[:k]) {
		return n, errors.New("xz: non-zero block padding")
	}
	checkSum := q[k:]
	computedSum := br.hash.Sum(checkSum[s:])
	if !bytes.Equal(checkSum, computedSum) {
		return n, errors.New("xz: checksum error for block")
	}
	return n, io.EOF
}

// Close closes the block reader and the LZMA2 reader supporting it.
func (br *blockReader) Close() error {
	if br.fr == nil {
		return errReaderClosed
	}
	if err := br.fr.Close(); err != nil {
		return err
	}
	br.fr = nil
	return nil
}

func (cfg *ReaderConfig) newFilterReader(r io.Reader, f []filter) (fr io.ReadCloser, err error) {

	if err = verifyFilters(f); err != nil {
		return nil, err
	}

	fr = io.NopCloser(r)
	for i := len(f) - 1; i >= 0; i-- {
		fr, err = f[i].reader(fr, cfg)
		if err != nil {
			return nil, err
		}
	}
	return fr, nil
}
