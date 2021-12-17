package lzma

import (
	"bytes"
	"io"
	"os"
	"testing"
)

func TestHeader(t *testing.T) {
	tests := []params{
		{Properties{3, 0, 2}, 1 << 15, eosSize},
	}
	for _, tc := range tests {
		s := tc.append(nil)
		var h params
		if err := h.parse(s); err != nil {
			t.Fatalf("h.parse error %s", err)
		}
		if h != tc {
			t.Fatalf("got %+v; want %+v", h, tc)
		}
	}
}

func TestReaderSimple(t *testing.T) {
	const file = "testdata/fox.lzma"
	const text = "The quick brown fox jumps over the lazy dog.\n"

	f, err := os.Open(file)
	if err != nil {
		t.Fatalf("os.Open(%q) error %s", file, err)
	}
	defer f.Close()

	r, err := NewReader(f)
	if err != nil {
		t.Fatalf("NewReader(f) error %s", err)
	}
	buf := new(bytes.Buffer)
	_, err = io.Copy(buf, r)
	if err != nil {
		t.Fatalf("io.Copy(buf, r) error %s", err)
	}
	s := buf.String()
	if s != text {
		t.Fatalf("got %q; want %q", s, text)
	}

	fi, err := f.Stat()
	if err != nil {
		t.Fatalf("f.State() error %s", err)
	}

	n, err := f.Seek(0, os.SEEK_CUR)
	if err != nil {
		t.Fatalf("f.Seek() error %s", err)
	}
	if n != fi.Size() {
		t.Fatalf("f pos %d; want eof pos = %d", n, fi.Size())
	}
}
