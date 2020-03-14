// Copyright 2014-2019 Ulrich Kunitz. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package xzinternals

import (
	"errors"
	"hash"
	"io"

	"github.com/ulikunitz/xz/filter"
)

// countingWriter is a writer that counts all data written to it.
type countingWriter struct {
	w io.Writer
	n int64
}

func NewCountingWriter(wr io.Writer) countingWriter {
	return countingWriter{w: wr}
}

// Write writes data to the countingWriter.
func (cw *countingWriter) Write(p []byte) (n int, err error) {
	n, err = cw.w.Write(p)
	cw.n += int64(n)
	if err == nil && cw.n < 0 {
		return n, errors.New("xz: counter overflow")
	}
	return
}

// BlockWriter is writes a single block.
type BlockWriter struct {
	CXZ countingWriter
	// MW combines io.WriteCloser w and the hash.
	MW        io.Writer
	W         io.WriteCloser
	n         int64
	BlockSize int64
	closed    bool
	headerLen int

	Filters []filter.Filter
	Hash    hash.Hash
}

// WriteHeader writes the header. If the function is called after Close
// the commpressedSize and uncompressedSize fields will be filled.
func (bw *BlockWriter) WriteHeader(w io.Writer) error {
	h := BlockHeader{
		CompressedSize:   -1,
		UncompressedSize: -1,
		Filters:          bw.Filters,
	}
	if bw.closed {
		h.CompressedSize = bw.compressedSize()
		h.UncompressedSize = bw.uncompressedSize()
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

// compressed size returns the amount of data written to the underlying
// stream.
func (bw *BlockWriter) compressedSize() int64 {
	return bw.CXZ.n
}

// uncompressedSize returns the number of data written to the
// blockWriter
func (bw *BlockWriter) uncompressedSize() int64 {
	return bw.n
}

// unpaddedSize returns the sum of the header length, the uncompressed
// size of the block and the hash size.
func (bw *BlockWriter) unpaddedSize() int64 {
	if bw.headerLen <= 0 {
		panic("xz: block header not written")
	}
	n := int64(bw.headerLen)
	n += bw.compressedSize()
	n += int64(bw.Hash.Size())
	return n
}

// Record returns the Record for the current stream. Call Close before
// calling this method.
func (bw *BlockWriter) Record() Record {
	return Record{bw.unpaddedSize(), bw.uncompressedSize()}
}

var ErrClosed = errors.New("xz: writer already closed")

var ErrNoSpace = errors.New("xz: no space")

// Write writes uncompressed data to the block writer.
func (bw *BlockWriter) Write(p []byte) (n int, err error) {
	if bw.closed {
		return 0, ErrClosed
	}

	t := bw.BlockSize - bw.n
	if int64(len(p)) > t {
		err = ErrNoSpace
		p = p[:t]
	}

	var werr error
	n, werr = bw.MW.Write(p)
	bw.n += int64(n)
	if werr != nil {
		return n, werr
	}
	return n, err
}

// Close closes the writer.
func (bw *BlockWriter) Close() error {
	if bw.closed {
		return ErrClosed
	}
	bw.closed = true
	if err := bw.W.Close(); err != nil {
		return err
	}
	s := bw.Hash.Size()
	k := padLen(bw.CXZ.n)
	p := make([]byte, k+s)
	bw.Hash.Sum(p[k:k])
	if _, err := bw.CXZ.w.Write(p); err != nil {
		return err
	}
	return nil
}
