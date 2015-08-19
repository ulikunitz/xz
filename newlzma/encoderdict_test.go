package newlzma

import (
	"fmt"
	"testing"
)

func TestEncoderDict(t *testing.T) {
	const (
		tst     = "abc abc abc"
		dictCap = 8
	)
	ht, err := newHashTable(dictCap, 3)
	if err != nil {
		t.Fatalf("newHashTable(%d, %d): error %s", dictCap, 3, err)
	}
	var e encoderDict
	if err = _initEncoderDict(&e, 8, 12, ht); err != nil {
		t.Fatalf("_initEncoderDict error %s", err)
	}
	if k := e.Available(); k != e.buf.Cap() {
		t.Fatalf("e.Available returned %d; want %d",
			k, e.buf.Cap())
	}
	n, err := e.Write([]byte(tst))
	if err != nil {
		t.Fatalf("Write error %s", err)
	}
	if n != len(tst) {
		t.Fatalf("Write returned %d; want %d", n, len(tst))
	}
	if k := e.Buffered(); k != len(tst) {
		t.Fatalf("e.Buffered returned %d; want %d", k, len(tst))
	}
	if k := e.Available(); k != e.buf.Cap()-len(tst) {
		t.Fatalf("e.Available#2 returned %d; want %d", k,
			e.buf.Cap()-len(tst))
	}
	if err = e.Advance(8); err != nil {
		t.Fatalf("e.Advance(%d) error %s", 8, err)
	}
	p := make([]byte, 3)
	n, err = e.buf.Peek(p)
	if err != nil {
		t.Fatalf("Peek error %s", err)
	}
	if n != len(p) {
		t.Fatalf("Peek returned %d; want %d", n, len(p))
	}
	dists, err := e.Matches()
	if err != nil {
		t.Fatalf("Matches error %s", err)
	}
	wdists := []int{8, 4}
	dstr, wdstr := fmt.Sprintf("%v", dists), fmt.Sprintf("%v", wdists)
	if dstr != wdstr {
		t.Fatalf("Matches returned %s; want %s", dstr, wdstr)
	}
	for _, d := range dists {
		n = e.MatchLen(d)
		if n != 3 {
			t.Fatalf("MatchLen(%d) returned %d; want %d", d, n, 3)
		}
	}
}
