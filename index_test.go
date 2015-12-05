package xz

import (
	"bytes"
	"testing"
)

func TestRecordReadWrite(t *testing.T) {
	r := record{1234567, 10000}
	var buf bytes.Buffer
	n, err := r.writeTo(&buf)
	if err != nil {
		t.Fatalf("writeTo error %s", err)
	}
	var g record
	m, err := g.readFrom(&buf)
	if err != nil {
		t.Fatalf("readFrom error %s", err)
	}
	if m != n {
		t.Fatalf("read %d bytes; wrote %d", m, n)
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
