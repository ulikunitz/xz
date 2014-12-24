package lzma

import (
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
