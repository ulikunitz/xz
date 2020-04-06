package xz

import (
	"bytes"
	"io"
	"os"
	"testing"
)

func TestReaderAtSimple(t *testing.T) {
	testFile(t, "testfiles/fox.xz")
	testFile(t, "testfiles/fox-check-none.xz")
}

func testFile(t *testing.T, file string) {
	xz, err := os.Open(file)
	if err != nil {
		t.Fatalf("os.Open(%q) error %s", file, err)
	}

	info, err := os.Stat(file)
	if err != nil {
		t.Fatalf("os.Stat(%q) error %s", file, err)
	}

	conf := ReaderAtConfig{
		Len: info.Size(),
	}
	r, err := conf.NewReaderAt(xz)
	if err != nil {
		t.Fatalf("NewReader error %s", err)
	}
	var buf bytes.Buffer
	reader := newRat(r, 0)
	if _, err = io.Copy(&buf, reader); err != nil {
		t.Fatalf("io.Copy error %s", err)
	}

	bufStr := buf.String()
	expected := "The quick brown fox jumps over the lazy dog.\n" // fixme
	if bufStr != expected {
		t.Fatalf("Unexpected decompression output. \"%s\" != \"%s\"", bufStr, expected)
	}
}
