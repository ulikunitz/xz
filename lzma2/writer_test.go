package lzma2

import (
	"bytes"
	"testing"
)

func TestWriter(t *testing.T) {
	var buf bytes.Buffer
	_, err := NewWriter(&buf)
	if err != nil {
		t.Fatalf("NewWriter error %s")
	}
}
