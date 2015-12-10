package xz

import (
	"bytes"
	"errors"
	"hash"
	"hash/crc64"
	"io"

	"github.com/ulikunitz/xz/lzma2"
)

var errUnexpectedEOF = errors.New("xz: unexpected end of file")

type ReaderParameters struct {
	flags byte
}

var ReaderDefaults = ReaderParameters{}

// Flags for the reader parameters.
const (
	Serial = 1 << iota
)

type Reader struct {
}

func NewReader(xz io.Reader) (r *Reader, err error) {
	return NewReaderParams(xz, ReaderDefaults)
}

func NewReaderParams(xz io.Reader, params ReaderParameters) (r *Reader, err error) {
	r = &Reader{}
	return r, nil
}

type blockReader struct {
	r     io.Reader
	lzma2 io.Reader
	hash  hash.Hash
	count int64
	size  int64
	err   error
}

var crc64Table = crc64.MakeTable(crc64.ECMA)

func newBlockReader(r io.Reader, newHash func() hash.Hash, bh *blockHeader) (br *blockReader, err error) {
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

	dc := f.(*lzmaFilter).dictCap
	dictCap := int(dc)
	if int64(dictCap) != dc {
		return nil, errors.New("xz: DictCap overflow")
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

var errBlockSize = errors.New("xz: wrong uncompressed size for block")

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

func (br *blockReader) Read(p []byte) (n int, err error) {
	if br.err != nil {
		return 0, br.err
	}
	n, err = br.read(p)
	br.err = err
	return n, err
}
