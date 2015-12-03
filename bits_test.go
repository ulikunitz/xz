package xz

import (
	"bytes"
	"testing"
)

func TestUvarint(t *testing.T) {
	tests := []uint64{0, 0x80, 0x100, 0xffffffff, 0x100000000, 1<<64 - 1}
	p := make([]byte, 10)
	for _, u := range tests {
		p = p[:10]
		n := putUvarint(p, u)
		if n < 1 {
			t.Fatalf("putUvarint returned %d", n)
		}
		r := bytes.NewReader(p[:n])
		x, err := readUvarint(r)
		if err != nil {
			t.Fatalf("readUvarint returned %s", err)
		}
		if x != u {
			t.Fatalf("readUvarint returned 0x%x; want 0x%x", x, u)
		}
	}
}
