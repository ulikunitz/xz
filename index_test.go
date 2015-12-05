package xz

import (
	"bytes"
	"testing"
)

func TestRecordReadWrite(t *testing.T) {
	r := record{1234567, 10000}
	var buf bytes.Buffer
	if err := r.writeTo(&buf); err != nil {
		t.Fatalf("writeTo error %s", err)
	}
	var g record
	if err := g.readFrom(&buf); err != nil {
		t.Fatalf("readFrom error %s", err)
	}
	if buf.Len() != 0 {
		t.Fatalf("buffer still has %d bytes", buf.Len())
	}
	if g.unpaddedSize != r.unpaddedSize {
		t.Fatalf("got unpaddedSize %d; want %d", g.unpaddedSize,
			r.unpaddedSize)
	}
	if g.uncompressedSize != r.uncompressedSize {
		t.Fatalf("got uncompressedSize %d; want %d", g.uncompressedSize,
			r.uncompressedSize)
	}
}
