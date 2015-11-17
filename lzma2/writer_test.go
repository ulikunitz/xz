package lzma2

import (
	"bytes"
	"testing"
)

func TestWriter(t *testing.T) {
	var buf bytes.Buffer
	w, err := NewWriter(&buf)
	if err != nil {
		t.Fatalf("NewWriter error %s", err)
	}
	if _, err = w.Write([]byte{'a'}); err != nil {
		t.Fatalf("w.Write([]byte{'a'}) error %s", err)
	}
	if err = w.Flush(); err != nil {
		t.Fatalf("w.Flush() error %s", err)
	}
	if err = w.Close(); err != nil {
		t.Fatalf("w.Close() error %s", err)
	}
	p := buf.Bytes()
	want := []byte{1, 0, 0, 'a'}
	if !bytes.Equal(p, want) {
		t.Fatalf("bytes written %#v; want %#v", p, want)
	}
}
