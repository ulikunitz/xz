// Package xz supports the compression and decompression of xz files.
package xz

import (
	"bytes"
	"errors"
	"hash"
	"io"

	"github.com/ulikunitz/xz/lzma2"
)

// errUnexpectedEOF indicates an unexpected end of file.
var errUnexpectedEOF = errors.New("xz: unexpected end of file")

// Reader decodes xz files.
type Reader struct {
	DictCap int

	xz      io.Reader
	br      *blockReader
	newHash func() hash.Hash
	h       header
}

// NewReader creates a new xz reader.
func NewReader(xz io.Reader) (r *Reader, err error) {
	r = &Reader{DictCap: 8 * 1024 * 1024, xz: xz}
	p := make([]byte, headerLen)
	if _, err = io.ReadFull(r.xz, p); err != nil {
		if err == io.EOF {
			err = errUnexpectedEOF
		}
		return nil, err
	}
	if err = r.h.UnmarshalBinary(p); err != nil {
		return nil, err
	}
	if r.newHash, err = newHashFunc(r.h.flags); err != nil {
		return nil, err
	}
	return r, nil
}

// readTail reads the index body and the xz footer.
func (r *Reader) readTail() error {
	_, n, err := readIndexBody(r.xz)
	if err != nil {
		if err == io.EOF {
			err = errUnexpectedEOF
		}
		return err
	}
	// TODO: check records
	p := make([]byte, footerLen)
	if _, err = io.ReadFull(r.xz, p); err != nil {
		if err == io.EOF {
			err = errUnexpectedEOF
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

// read reads actual data from the xz stream.
func (r *Reader) Read(p []byte) (n int, err error) {
	for n < len(p) {
		if r.br == nil {
			bh, _, err := readBlockHeader(r.xz)
			if err != nil {
				if err == errIndexIndicator {
					if err = r.readTail(); err != nil {
						return n, err
					}
					return n, io.EOF
				}
				return n, err
			}
			r.br, err = newBlockReader(r.xz, bh, r.newHash(),
				r.DictCap)
			if err != nil {
				return n, err
			}
		}
		k, err := r.br.Read(p[n:])
		n += k
		if err != nil {
			if err == io.EOF {
				r.br = nil
			} else {
				return n, err
			}
		}
	}
	return n, nil
}

// lenReader counts the number of bytes read.
type lenReader struct {
	r io.Reader
	n int64
}

// Read reads data from the wrapped reader and adds it to the n field.
func (lr *lenReader) Read(p []byte) (n int, err error) {
	n, err = lr.r.Read(p)
	lr.n += int64(n)
	return n, err
}

// blockReader supports the reading of a block.
type blockReader struct {
	lxz    lenReader
	header *blockHeader
	n      int64
	hash   hash.Hash
	r      io.Reader
	err    error
}

// newBlockReader creates a new block reader.
func newBlockReader(xz io.Reader, h *blockHeader, hash hash.Hash, dictCap int) (br *blockReader, err error) {
	br = &blockReader{
		lxz:    lenReader{r: xz},
		header: h,
		hash:   hash,
	}

	f := h.filters[0].(*lzmaFilter)
	dc := int(f.dictCap)
	if dc < 0 {
		return nil, errors.New("xz: dictionary capacity overflow")
	}
	if dictCap < dc {
		dictCap = dc
	}
	lr, err := lzma2.NewReader(&br.lxz, dictCap)
	if err != nil {
		return nil, err
	}
	br.r = io.TeeReader(lr, br.hash)

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

// errBlockSize indicates that the size of the block in the block header
// is wrong.
var errBlockSize = errors.New("xz: wrong uncompressed size for block")

// Read reads data from the block.
func (br *blockReader) Read(p []byte) (n int, err error) {
	n, err = br.r.Read(p)
	br.n += int64(n)
	if br.header.uncompressedSize >= 0 &&
		br.n > br.header.uncompressedSize {
		return n, errBlockSize
	}
	if err != io.EOF {
		return n, err
	}
	if br.n < br.header.uncompressedSize {
		return n, errUnexpectedEOF
	}
	if br.header.compressedSize >= 0 &&
		br.lxz.n != br.header.compressedSize {
		return n, errUnexpectedEOF
	}
	s := br.hash.Size()
	k := padLen(br.n)
	q := make([]byte, k+s, k+2*s)
	if _, err = io.ReadFull(br.lxz.r, q); err != nil {
		if err == io.EOF {
			err = errUnexpectedEOF
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
