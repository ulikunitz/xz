// Package lzma2 provides readers and writers for the LZMA2 format. The
// format adds the capabilities flushing, parallel compression and
// uncompressed segments to the LZMA algorithm.
package lzma2

import "io"

type segmentReader struct {
}

func newSegmentReader(lzma io.Reader, p Parameters) (r *segmentReader, err error) {
	panic("TODO")
}

func (r *segmentReader) Read(p []byte) (n int, err error) {
	panic("TODO")
}

func (r *segmentReader) EOS() bool {
	panic("TODO")
}
