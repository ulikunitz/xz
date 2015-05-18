package lzb

import (
	"bytes"
	"io"
	"os"
	"testing"
)

func TestReader(t *testing.T) {
	filename := "fox.lzma"
	f, err := os.Open(filename)
	if err != nil {
		t.Fatalf("os.Open(%q) error %s", filename, err)
	}
	defer f.Close()
	p := make([]byte, 13)
	_, err = io.ReadFull(f, p)
	if err != nil {
		t.Fatalf("io.Readfull error %s", err)
	}
	params := Params{Properties: Properties(p[0]),
		DictSize: 0x800000}
	params.BufferSize = params.DictSize
	r, err := NewReader(f, params)
	if err != nil {
		t.Fatalf("NewReader error %s", err)
	}
	buf := &bytes.Buffer{}
	want := "The quick brown fox jumps over the lazy dog.\n"
	if _, err = io.Copy(buf, r); err != nil {
		t.Fatalf("Copy error %s", err)
	}
	got := buf.String()
	if got != want {
		t.Fatalf("read %q; but want %q", got, want)
	}
}
