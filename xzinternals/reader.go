// Copyright 2014-2019 Ulrich Kunitz. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
package xzinternals

import (
	"bytes"
	"errors"
	"fmt"
	"hash"
	"io"

	"github.com/ulikunitz/xz/filter"
	"github.com/ulikunitz/xz/internal/xlog"
)

// StreamReader decodes a single xz stream
type StreamReader struct {
	//	ReaderConfig
	dictCap int

	xz      io.Reader
	br      *BlockReader
	newHash func() hash.Hash
	h       Header
	index   []Record
}

// errIndex indicates an error with the xz file index.
var errIndex = errors.New("xz: error in xz file index")

// readTail reads the index body and the xz footer.
func (r *StreamReader) readTail() error {
	index, n, err := readIndexBody(r.xz)
	if err != nil {
		if err == io.EOF {
			err = io.ErrUnexpectedEOF
		}
		return err
	}
	if len(index) != len(r.index) {
		return fmt.Errorf("xz: index length is %d; want %d",
			len(index), len(r.index))
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
	var f Footer
	if err = f.UnmarshalBinary(p); err != nil {
		return err
	}
	xlog.Debugf("xz footer %s", f)
	if f.Flags != r.h.Flags {
		return errors.New("xz: footer flags incorrect")
	}
	if f.IndexSize != int64(n)+1 {
		return errors.New("xz: index size in footer wrong")
	}
	return nil
}

// Read reads actual data from the xz stream.
func (r *StreamReader) Read(p []byte) (n int, err error) {
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
			xlog.Debugf("block %v", *bh)
			r.br, err = NewBlockReader(r.xz, bh,
				hlen, r.newHash(), r.dictCap)
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
type BlockReader struct {
	lxz       countingReader
	header    *BlockHeader
	headerLen int
	n         int64
	hash      hash.Hash
	r         io.Reader
	err       error
}

// NewBlockReader creates a new block reader.
func NewBlockReader(xz io.Reader, h *BlockHeader,
	hlen int, hash hash.Hash, dictCap int) (br *BlockReader, err error) {

	br = &BlockReader{
		lxz:       countingReader{r: xz},
		header:    h,
		headerLen: hlen,
		hash:      hash,
	}

	config := filter.ReaderConfig{
		DictCap: dictCap,
	}

	fr, err := filter.NewFilterReader(&config, &br.lxz, h.Filters)
	if err != nil {
		return nil, err
	}
	if br.hash.Size() != 0 {
		br.r = io.TeeReader(fr, br.hash)
	} else {
		br.r = fr
	}

	return br, nil
}

// uncompressedSize returns the uncompressed size of the block.
func (br *BlockReader) uncompressedSize() int64 {
	return br.n
}

// compressedSize returns the compressed size of the block.
func (br *BlockReader) compressedSize() int64 {
	return br.lxz.n
}

// unpaddedSize computes the unpadded size for the block.
func (br *BlockReader) unpaddedSize() int64 {
	n := int64(br.headerLen)
	n += br.compressedSize()
	n += int64(br.hash.Size())
	return n
}

// record returns the index record for the current block.
func (br *BlockReader) record() Record {
	return Record{br.unpaddedSize(), br.uncompressedSize()}
}

// errBlockSize indicates that the size of the block in the block header
// is wrong.
var errBlockSize = errors.New("xz: wrong uncompressed size for block")

// Read reads data from the block.
func (br *BlockReader) Read(p []byte) (n int, err error) {
	n, err = br.r.Read(p)
	br.n += int64(n)

	u := br.header.UncompressedSize
	if u >= 0 && br.uncompressedSize() > u {
		return n, errors.New("xz: wrong uncompressed size for block")
	}
	c := br.header.CompressedSize
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

// NewStreamReader creates a new xz stream reader using the given configuration
// parameters. NewReader reads and checks the header of the xz stream.
func NewStreamReader(xz io.Reader, dictCap int) (r *StreamReader, err error) {
	data := make([]byte, HeaderLen)
	if _, err := io.ReadFull(xz, data[:4]); err != nil {
		return nil, err
	}
	if bytes.Equal(data[:4], []byte{0, 0, 0, 0}) {
		return nil, ErrPadding
	}
	if _, err = io.ReadFull(xz, data[4:]); err != nil {
		if err == io.EOF {
			err = io.ErrUnexpectedEOF
		}
		return nil, err
	}
	r = &StreamReader{
		dictCap: dictCap,
		xz:      xz,
		index:   make([]Record, 0, 4),
	}
	if err = r.h.UnmarshalBinary(data); err != nil {
		return nil, err
	}
	xlog.Debugf("xz header %s", r.h)
	if r.newHash, err = NewHashFunc(r.h.Flags); err != nil {
		return nil, err
	}
	return r, nil
}
