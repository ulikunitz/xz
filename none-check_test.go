package xz

import (
	"bytes"
	"testing"
)

func TestNoneHash(t *testing.T) {
	h := newNoneHash()

	p := []byte("foo")
	q := h.Sum(p)

	if !bytes.Equal(q, p) {
		t.Fatalf("h.Sum: got %q; want %q", q, p)
	}

}
