package xz

import (
	"bytes"
	"io"
	"testing"
)

func TestWriter(t *testing.T) {
	const text = "The quick brown fox jumps over the lazy dog."
	var buf bytes.Buffer
	w := NewWriter(&buf)
	n, err := io.WriteString(w, text)
	if err != nil {
		t.Fatalf("WriteString error %s", err)
	}
	if n != len(text) {
		t.Fatalf("Writestring wrote %d bytes; want %d", n, len(text))
	}
	if err = w.Close(); err != nil {
		t.Fatalf("w.Close error %s", err)
	}
	var out bytes.Buffer
	r, err := NewReader(&buf)
	if err != nil {
		t.Fatalf("NewReader error %s", err)
	}
	if _, err = io.Copy(&out, r); err != nil {
		t.Fatalf("io.Copy error %s", err)
	}
	s := out.String()
	if s != text {
		t.Fatalf("reader decompressed to %q; want %q", s, text)
	}
}
