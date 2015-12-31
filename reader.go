// Package xz supports the compression and decompression of xz files.
package xz

import (
	"bytes"
	"errors"
	"hash"
	"io"

	"github.com/ulikunitz/xz/lzma2"
)

// ReaderParams defines the parameters for the xz reader.
type ReaderParams struct {
	lzma2.ReaderParams
}

// ReaderDefaults defines the defaults for the xz reader.
var ReaderDefaults = ReaderParams{
	ReaderParams: lzma2.ReaderDefaults,
}

// errUnexpectedEOF indicates an unexpected end of file.
var errUnexpectedEOF = errors.New("xz: unexpected end of file")

// Reader decodes xz files.
type Reader struct {
	ReaderParams

	xz      io.Reader
	br      *blockReader
	newHash func() hash.Hash
	h       header
	index   []record
}

// NewReader creates a new xz reader.
func NewReader(xz io.Reader) (r *Reader, err error) {
	r = &Reader{
		xz:    xz,
		index: make([]record, 0, 4),
	}
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
	r.index = make([]record, 0, 4)
	return r, nil
}

// errIndex indicates an error with the xz file index.
var errIndex = errors.New("xz: error in xz file index")

// readTail reads the index body and the xz footer.
func (r *Reader) readTail() error {
	index, n, err := readIndexBody(r.xz)
	if err != nil {
		if err == io.EOF {
			err = errUnexpectedEOF
		}
		return err
	}
	if len(index) != len(r.index) {
		return errIndex
	}
	for i, rec := range r.index {
		if rec != index[i] {
			return errIndex
		}
	}

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
			r.br, err = newBlockReader(r.xz, bh, hlen, r.newHash(),
				r.DictCap)
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
	lxz       lenReader
	header    *blockHeader
	headerLen int
	n         int64
	hash      hash.Hash
	r         io.Reader
	err       error
}

// newBlockReader creates a new block reader.
func newBlockReader(xz io.Reader, h *blockHeader, hlen int, hash hash.Hash, dictCap int) (br *blockReader, err error) {
	br = &blockReader{
		lxz:       lenReader{r: xz},
		header:    h,
		headerLen: hlen,
		hash:      hash,
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

// errBlockSize indicates that the size of the block in the block header
// is wrong.
var errBlockSize = errors.New("xz: wrong uncompressed size for block")

// Read reads data from the block.
func (br *blockReader) Read(p []byte) (n int, err error) {
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
