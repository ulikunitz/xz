package xz

import (
	"bytes"
	"io"
	"testing"
)

func TestHashReader(t *testing.T) {
	b := []byte{0x00, 0x04}
	r := newCRC32Reader(bytes.NewReader(b))
	buf := make([]byte, 2)
	n, err := io.ReadFull(r, buf)
	if err != nil {
		t.Fatalf("io.ReadFull: %s", err)
	}
	if n != 2 {
		t.Fatalf("io.ReadFull: n=%d; want %d", n, 2)
	}
	checksum := r.Sum(nil)
	t.Logf("checksum: %#v", checksum)
}
