package lzma2

import "io"

type segmentReader struct {
}

type segmentParameters struct {
	DictSize int64
}

func newSegmentReader(lzma io.Reader, p segmentParameters) (r *segmentReader, err error) {
	panic("TODO")
}

func (r *segmentReader) Read(p []byte) (n int, err error) {
	panic("TODO")
}

func (r *segmentReader) EOS() bool {
	panic("TODO")
}
