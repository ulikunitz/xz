package lzma

import (
	"os"
	"testing"
)

func TestOldHeader(t *testing.T) {
	f, err := os.Open("examples/a.lzma")
	if err != nil {
		t.Fatalf("os.Open(\"examples/a.lzma\"): %v", err)
	}
	p, size, err := readOldHeader(f)
	if err != nil {
		t.Fatalf("readOldHeader: %v", err)
	}
	t.Logf("p: %#v", p)
	if p.PB != 2 {
		t.Errorf("pb %d; want 2", p.PB)
	}
	if p.LP != 0 {
		t.Errorf("lp %d; want 0", p.LP)
	}
	if p.LC != 3 {
		t.Errorf("lc %d; want 3", p.LC)
	}
	if size != 327 {
		t.Errorf("size %d; want 327", size)
	}
}
