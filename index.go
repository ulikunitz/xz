package xz

import (
	"errors"
	"io"
)

// record describes a block in the xz file index.
type record struct {
	unpaddedSize     int64
	uncompressedSize int64
}

// readFrom reads the record from the byte reader
func (rec *record) readFrom(r io.ByteReader) (n int, err error) {
	u, k, err := readUvarint(r)
	n += k
	if err != nil {
		return n, err
	}
	rec.unpaddedSize = int64(u)
	if rec.unpaddedSize < 0 {
		return n, errors.New("xz: unpadded size negative")
	}
	u, k, err = readUvarint(r)
	n += k
	if err != nil {
		return n, err
	}
	rec.uncompressedSize = int64(u)
	if rec.uncompressedSize < 0 {
		return n, errors.New("xz: uncompressed size negative")
	}
	return n, nil
}

// writeTo writes the record into the writer
func (rec *record) writeTo(w io.Writer) (n int, err error) {
	// maximum length of a uvarint is 10
	p := make([]byte, 20)
	n = putUvarint(p, uint64(rec.unpaddedSize))
	n += putUvarint(p[n:], uint64(rec.uncompressedSize))
	return w.Write(p[:n])
}
