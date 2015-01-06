package lzma

import (
	"fmt"
	"testing"
)

func TestSlot(t *testing.T) {
	e := make([]uint32, slotEntries+10)
	for i := range e {
		e[i] = uint32(i * i)
	}
	var s slot
	for _, p := range e {
		s.putEntry(p)
	}
	r := s.getEntries()
	if len(r) != slotEntries {
		t.Fatalf("len(r) %d; want %d", len(r), slotEntries)
	}
	d := e[len(e)-slotEntries:]
	for i, p := range r {
		q := d[i]
		if p != q {
			t.Fatalf("r[%d]=%d unexpected; want %d", i, p, q)
		}
	}
}

func TestHashTable(t *testing.T) {
	ht := newHashTable(9, 2)
	s := "abcabcdefghijklmn"
	n, err := ht.Write([]byte(s))
	if err != nil {
		t.Fatalf("ht.Write: error %s", err)
	}
	if n != len(s) {
		t.Fatalf("ht.Write returned %d; want %d", n, len(s))
	}
	tests := []struct {
		s string
		w string
	}{
		{"ab", "[0 3]"},
		{"bc", "[1 4]"},
		{"ca", "[2]"},
		{"xx", "[]"},
		{"gh", "[9]"},
	}
	for _, c := range tests {
		offs := ht.Offsets([]byte(c.s))
		o := fmt.Sprintf("%v", offs)
		if o != c.w {
			t.Errorf("%s: offsets %s; want %s", c.s, o, c.w)
		}
	}
}
