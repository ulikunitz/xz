// Package xz supports the compression and decompression of xz files.
package xz

import (
	"bytes"
	"errors"
	"fmt"
	"hash"
	"io"

	"github.com/ulikunitz/xz/lzma"
	"github.com/ulikunitz/xz/lzma2"
)

// errUnexpectedEOF indicates an unexpected end of file.
var errUnexpectedEOF = errors.New("xz: unexpected end of file")

// Reader decodes xz files.
type Reader struct {
	DictCap int

	dictCap int
	xz      io.Reader
	err     error
	br      *blockReader
	newHash func() hash.Hash
	h       header
}

// NewReader creates a new xz reader.
func NewReader(xz io.Reader) (r *Reader, err error) {
	if xz == nil {
		return nil, errors.New("xz: reader must be not nil")
	}
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
func (r *Reader) read(p []byte) (n int, err error) {
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
			r.br, err = newBlockReader(r.xz, r.newHash, bh,
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

// Read decompresses the data of the xz stream.
func (r *Reader) Read(p []byte) (n int, err error) {
	if r.err != nil {
		return 0, r.err
	}
	n, err = r.read(p)
	r.err = err
	return n, err
}

// blockReader is used to read data from a single block.
type blockReader struct {
	r     io.Reader
	lzma2 io.Reader
	hash  hash.Hash
	count int64
	size  int64
	err   error
}

// newBlockReader creates a new block reader.
func newBlockReader(r io.Reader, newHash func() hash.Hash, bh *blockHeader, dictCap int) (br *blockReader, err error) {
	// some consistency checks
	if len(bh.filters) != 1 {
		return nil, errors.New("xz: multiple filters are unsupported")
	}
	f := bh.filters[0]
	if f.id() != lzmaFilterID {
		return nil, errors.New("xz: filter id unsupported")
	}

	br = &blockReader{
		hash: newHash(),
	}

	if bh.compressedSize < 0 {
		br.r = r
	} else {
		br.r = io.LimitReader(r, bh.compressedSize)
	}
	if bh.uncompressedSize < 0 {
		br.size = -1
	} else {
		br.size = bh.uncompressedSize
	}

	udc := f.(*lzmaFilter).dictCap
	dc := int(udc)
	if int64(dc) != udc {
		return nil, errors.New("xz: DictCap overflow")
	}
	if dc < dictCap {
		dictCap = dc
	}

	br.lzma2, err = lzma2.NewReader(r, dictCap)
	if err != nil {
		if err == io.EOF {
			err = errUnexpectedEOF
		}
		return nil, err
	}
	br.lzma2 = io.TeeReader(br.lzma2, br.hash)

	return br, nil
}

// Properties returns the properties currently used for decoding.
func (r *Reader) Properties() (props lzma.Properties, ok bool) {
	if r.br == nil || r.br.lzma2 == nil {
		return lzma.Properties{}, false
	}
	lr := r.br.lzma2.(*lzma2.Reader)
	return lr.Properties()
}

// errBlockSize indicates that the size of the block in the block header
// is wrong.
var errBlockSize = errors.New("xz: wrong uncompressed size for block")

// read reads data from the block and checks the checksum at the end.
func (br *blockReader) read(p []byte) (n int, err error) {
	n, err = br.lzma2.Read(p)
	br.count += int64(n)
	if br.size >= 0 && br.count > br.size {
		return n, errBlockSize
	}
	if err != io.EOF {
		return n, err
	}
	if br.count < br.size {
		return n, errUnexpectedEOF
	}
	k := int(br.count % 4)
	if k > 0 {
		k = 4 - k
	}
	s := br.hash.Size()
	q := make([]byte, k+s, k+2*s)
	if _, err = io.ReadFull(br.r, q); err != nil {
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

// Read reads uncompressed data from the block.
func (br *blockReader) Read(p []byte) (n int, err error) {
	if br.err != nil {
		fmt.Printf("Repeated block read %d error %v\n", 0, br.err)
		return 0, br.err
	}
	n, err = br.read(p)
	br.err = err
	return n, err
}
