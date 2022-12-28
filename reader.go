// Copyright 2014-2022 Ulrich Kunitz. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package xz supports the compression and decompression of xz files. It
// supports version 1.1.0 of the specification without the non-LZMA2
// filters. See http://tukaani.org/xz/xz-file-format-1.1.0.txt
package xz

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"hash"
	"io"
	"runtime"

	"github.com/ulikunitz/xz/lzma"
)

var errReaderClosed = errors.New("xz: reader closed")
var errUnexpectedData = errors.New("xz: unexpected Data after stream")

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

	// Workers defines the number of readers for parallel reading. The
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

	if cfg.Workers < 1 {
		return errors.New("xz: reader workers must be >= 1")
	}

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

type streamReader interface {
	io.ReadCloser
	reset(hdr *header) error
}

// reader supports the reading of one or multiple xz streams.
type reader struct {
	cfg ReaderConfig

	xz io.Reader
	sr streamReader

	err error
}

// NewReader creates an io.ReadCloser. The function should never fail.
func NewReader(xz io.Reader) (r io.ReadCloser, err error) {
	r, err = NewReaderConfig(xz, ReaderConfig{})
	if err != nil {
		return nil, err
	}
	return r, nil
}

func NewReaderConfig(xz io.Reader, cfg ReaderConfig) (r io.ReadCloser, err error) {
	cfg.ApplyDefaults()
	if err = cfg.Verify(); err != nil {
		return nil, err
	}

	rp := &reader{cfg: cfg}

	// for the single thread reader we are buffering
	rp.xz = bufio.NewReader(xz)
	rp.sr = newSingleThreadStreamReader(rp.xz, &rp.cfg)

	// read header without padding
	hdr, err := readHeader(rp.xz, false)
	if err != nil {
		return nil, err
	}
	if err = rp.sr.reset(hdr); err != nil {
		return nil, err
	}
	return rp, err
}

func (r *reader) Read(p []byte) (n int, err error) {
	if r.err != nil {
		return 0, r.err
	}
	for n < len(p) {
		k, err := r.sr.Read(p[n:])
		n += k
		if err != nil {
			if err == io.EOF {
				if err = r.sr.Close(); err != nil {
					r.err = err
					return n, err
				}
				if r.cfg.SingleStream {
					var q [1]byte
					_, err = io.ReadFull(r.xz, q[:1])
					if err == nil {
						err = errUnexpectedData
					} else if err == io.ErrUnexpectedEOF {
						err = io.EOF
					}
					r.err = err
					return n, err
				}
				// read header with padding
				hdr, err := readHeader(r.xz, true)
				if err != nil {
					r.err = err
					return n, err
				}
				if err = r.sr.reset(hdr); err != nil {
					r.err = err
					return n, err
				}
				continue
			}
			r.err = err
			return n, err
		}
	}
	return n, nil
}

func (r *reader) Close() error {
	if r.err == errReaderClosed {
		return errReaderClosed
	}
	if err := r.sr.Close(); err != nil && err != errReaderClosed {
		r.err = err
		return err
	}
	r.err = errReaderClosed
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
	cfg *ReaderConfig

	hash hash.Hash

	header    *blockHeader
	headerLen int

	xz           io.Reader
	cxz          countingReader
	fr           io.ReadCloser
	r            io.Reader
	uncompressed int64

	err error
}

func (br *blockReader) init(xz io.Reader, cfg *ReaderConfig, h hash.Hash) {
	*br = blockReader{
		cfg:  cfg,
		xz:   xz,
		hash: h,
	}
	h.Reset()
}

func (br *blockReader) reset() {
	*br = blockReader{
		cfg:  br.cfg,
		xz:   br.xz,
		hash: br.hash,
	}
	br.hash.Reset()
}

func (br *blockReader) setHeader(hdr *blockHeader, hdrLen int) error {
	if br.err != nil {
		return br.err
	}
	if br.header != nil {
		return errors.New("xz: header already set")
	}
	br.header = hdr
	br.headerLen = hdrLen

	br.cxz = countingReader{r: br.xz}

	var err error
	br.fr, err = br.cfg.newFilterReader(&br.cxz, hdr.filters)
	if err != nil {
		br.err = err
		return err
	}
	if br.hash.Size() != 0 {
		br.r = io.TeeReader(br.fr, br.hash)
	} else {
		br.r = br.fr
	}

	return nil
}

// unpaddedSize computes the unpadded size for the block.
func (br *blockReader) unpaddedSize() int64 {
	n := int64(br.headerLen)
	n += br.cxz.n
	n += int64(br.hash.Size())
	return n
}

// record returns the index record for the current block.
func (br *blockReader) record() record {
	return record{br.unpaddedSize(), br.uncompressed}
}

var errUnexpectedEndOfBlock = errors.New("xz: unexpected end of block")

// Read reads data from the block.
func (br *blockReader) Read(p []byte) (n int, err error) {
	if br.err != nil {
		return 0, br.err
	}

	if br.header == nil {
		hdr, hdrLen, err := readBlockHeader(br.xz)
		if err != nil {
			br.err = err
			return 0, err
		}
		if err = br.setHeader(hdr, hdrLen); err != nil {
			br.err = err
			return 0, err
		}
	}

	n, err = br.r.Read(p)
	br.uncompressed += int64(n)

	u := br.header.uncompressedSize
	if u >= 0 && br.uncompressed > u {
		br.err = errors.New("xz: wrong uncompressed size for block")
		return n, br.err
	}
	c := br.header.compressedSize
	if c >= 0 && br.cxz.n > c {
		br.err = errors.New("xz: wrong compressed size for block")
		return n, br.err
	}
	if err != io.EOF {
		if err != nil {
			br.err = err
		}
		return n, err
	}

	// EOF of the LZMA stream
	if br.uncompressed < u || br.cxz.n < c {
		br.err = errUnexpectedEndOfBlock
		return n, br.err
	}

	s := br.hash.Size()
	k := padLen(br.cxz.n)
	q := make([]byte, k+s, k+2*s)
	if _, err = io.ReadFull(br.cxz.r, q); err != nil {
		if err == io.EOF {
			err = io.ErrUnexpectedEOF
		}
		br.err = err
		return n, err
	}
	if !allZeros(q[:k]) {
		br.err = errors.New("xz: non-zero block padding")
		return n, br.err
	}
	checkSum := q[k:]
	computedSum := br.hash.Sum(checkSum[s:])
	if !bytes.Equal(checkSum, computedSum) {
		br.err = errors.New("xz: checksum error for block")
		return n, br.err
	}

	br.err = io.EOF
	return n, io.EOF
}

// Close closes the block reader and the LZMA2 reader supporting it.
func (br *blockReader) Close() error {
	if br.err == errReaderClosed {
		return errReaderClosed
	}
	if br.fr != nil {
		if err := br.fr.Close(); err != nil {
			br.err = err
			return err
		}
	}
	br.err = errReaderClosed
	return nil
}

type stReader struct {
	cfg *ReaderConfig
	xz  io.Reader

	br    blockReader
	index []record
	flags byte

	err error
}

func newSingleThreadStreamReader(xz io.Reader, cfg *ReaderConfig) streamReader {
	return &stReader{cfg: cfg, xz: xz}
}

func (sr *stReader) reset(hdr *header) error {
	h, err := newHash(hdr.flags)
	if err != nil {
		return err
	}
	*sr = stReader{
		cfg:   sr.cfg,
		xz:    sr.xz,
		flags: hdr.flags,
	}
	sr.br.init(sr.xz, sr.cfg, h)
	return nil
}

func (sr *stReader) Read(p []byte) (n int, err error) {
	if sr.err != nil {
		return 0, sr.err
	}
	for n < len(p) {
		k, err := sr.br.Read(p[n:])
		n += k
		if err != nil {
			if err == io.EOF {
				sr.index = append(sr.index, sr.br.record())
				if err = sr.br.Close(); err != nil {
					sr.err = err
					return n, err
				}
				sr.br.reset()
				continue
			}
			if err == errIndexIndicator {
				err = readTail(sr.xz, sr.index, sr.flags)
				if err != nil {
					sr.err = err
					return n, err
				}
				err = io.EOF
			}
			sr.err = err
			return n, err
		}
	}

	return n, nil
}

func (sr *stReader) Close() error {
	if sr.err == errReaderClosed {
		return errReaderClosed
	}
	if err := sr.br.Close(); err != nil {
		sr.err = err
		return err
	}
	sr.err = errReaderClosed
	return nil
}

// readHeader reads header from the reader and skips padding if the padding
// argument is true. A possible outcome is io. EOF. If there is a problem with
// the padding errPadding is returned.
func readHeader(r io.Reader, padding bool) (hdr *header, err error) {
	p := make([]byte, HeaderLen)
	if padding {
	loop:
		for {
			n, err := io.ReadFull(r, p)
			if err != nil {
				if err == io.ErrUnexpectedEOF {
					if allZeros(p[:n]) {
						if n%4 != 0 {
							return nil, errPadding
						}
						return nil, io.EOF
					}
				}
				return nil, err
			}
			for i, b := range p {
				if b != 0 {
					if i == 0 {
						break loop
					}
					if i%4 != 0 {
						return nil, errPadding
					}
					n = copy(p, p[i:])
					_, err = io.ReadFull(r, p[n:])
					if err != nil {
						return nil, err
					}
					break loop
				}
			}
		}
	} else {
		_, err = io.ReadFull(r, p)
		if err != nil {
			return nil, err
		}
	}
	hdr = new(header)
	if err = hdr.UnmarshalBinary(p); err != nil {
		return nil, err
	}
	return hdr, nil
}

// readTail reads the index body and the xz footer.
func readTail(xz io.Reader, rindex []record, flags byte) error {
	index, n, err := readIndexBody(xz, len(rindex))
	if err != nil {
		if err == io.EOF {
			err = io.ErrUnexpectedEOF
		}
		return err
	}

	for i, rec := range index {
		if rec != rindex[i] {
			return fmt.Errorf("xz: record %d is %v; want %v",
				i, rec, rindex[i])
		}
	}

	p := make([]byte, footerLen)
	if _, err = io.ReadFull(xz, p); err != nil {
		if err == io.EOF {
			err = io.ErrUnexpectedEOF
		}
		return err
	}
	var f footer
	if err = f.UnmarshalBinary(p); err != nil {
		return err
	}
	if f.flags != flags {
		return errors.New("xz: footer flags incorrect")
	}
	if f.indexSize != int64(n)+1 {
		return errors.New("xz: index size in footer wrong")
	}
	return nil
}
