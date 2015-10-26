package lzma2

import (
	"bytes"
	"testing"
)

func TestWriteEOS(t *testing.T) {
	buf := new(bytes.Buffer)
	err := WriteEOS(buf)
	if err != nil {
		t.Fatalf("WriteEOS: unexpected error %s", err)
	}
	b := buf.Bytes()
	if len(b) != 1 {
		t.Fatalf("len(b) is %d; want %d", len(b), 1)
	}
	if b[0] != 0 {
		t.Fatalf("b[0] is %#4x; want %#4x", b[0], byte(0))
	}
}
