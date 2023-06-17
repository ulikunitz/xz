package lzma

import (
	"io"
)

func limitByteReader(br io.ByteReader, n int64) *limitedByteReader {
	return &limitedByteReader{
		BR: br,
		N:  n,
	}
}

type limitedByteReader struct {
	BR io.ByteReader
	N  int64
}

func (r *limitedByteReader) ReadByte() (b byte, err error) {
	if r.N <= 0 {
		return 0, io.EOF
	}
	b, err = r.BR.ReadByte()
	if err == nil {
		r.N -= 1
	}
	return
}
