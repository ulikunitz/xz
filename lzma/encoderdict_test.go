package lzma

import (
	"fmt"
	"testing"
)

func TestEncoderDict(t *testing.T) {
	const (
		tst     = "abc abc abc"
		bufCap  = 12
		dictCap = 8
	)
	e, err := NewEncoderDict(dictCap, bufCap)
	if err != nil {
		t.Fatalf("NewEncoderDict error %s", err)
	}
	// default matcher wordLen is 4; so this is patched here
	if e.m, err = newHashTable(dictCap, 3); err != nil {
		t.Fatalf("newHashTable error %s", err)
	}
	n, err := e.write([]byte(tst))
	if err != nil {
		t.Fatalf("Write error %s", err)
	}
	if n != len(tst) {
		t.Fatalf("Write returned %d; want %d", n, len(tst))
	}
	if k := e.Buffered(); k != len(tst) {
		t.Fatalf("e.Buffered returned %d; want %d", k, len(tst))
	}
	e.advance(8)
	p := make([]byte, 3)
	n, err = e.buf.Peek(p)
	if err != nil {
		t.Fatalf("Peek error %s", err)
	}
	if n != len(p) {
		t.Fatalf("Peek returned %d; want %d", n, len(p))
	}
	dists := e.matches()
	wdists := []int{4, 8}
	dstr, wdstr := fmt.Sprintf("%v", dists), fmt.Sprintf("%v", wdists)
	if dstr != wdstr {
		t.Fatalf("Matches returned %s; want %s", dstr, wdstr)
	}
	for _, d := range dists {
		n = e.matchLen(d)
		if n != 3 {
			t.Fatalf("MatchLen(%d) returned %d; want %d", d, n, 3)
		}
	}
}
