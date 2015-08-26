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
	var buf encoderBuffer
	if err := initBuffer(&buf.buffer, 12); err != nil {
		t.Fatalf("initBuffer error %s", err)
	}
	var err error
	if buf.matcher, err = newHashTable(bufCap, 3); err != nil {
		t.Fatalf("newHashTable(%d, %d): error %s", dictCap, 3, err)
	}
	var e encoderDict
	_initEncoderDict(&e, 8, &buf)
	n, err := buf.Write([]byte(tst))
	if err != nil {
		t.Fatalf("Write error %s", err)
	}
	if n != len(tst) {
		t.Fatalf("Write returned %d; want %d", n, len(tst))
	}
	if k := e.Buffered(); k != len(tst) {
		t.Fatalf("e.Buffered returned %d; want %d", k, len(tst))
	}
	if err = e.Advance(8); err != nil {
		t.Fatalf("e.AdvanceHead(%d) error %s", 8, err)
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
	wdists := []int{4, 8}
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
