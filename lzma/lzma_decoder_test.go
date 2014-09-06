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
	var o oldHeader
	if err = o.read(f); err != nil {
		t.Fatalf("oldHeader read: %v", err)
	}
	t.Logf("o: %#v", o)
	if o.PB != 2 {
		t.Errorf("pb %d; want 2", o.PB)
	}
	if o.LP != 0 {
		t.Errorf("lp %d; want 0", o.LP)
	}
	if o.LC != 3 {
		t.Errorf("lc %d; want 3", o.LC)
	}
	if o.size != 327 {
		t.Errorf("size %d; want 327", o.size)
	}
}
