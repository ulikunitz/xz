package lzma

import (
	"testing"

	"github.com/uli-go/xz/hash"
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

func TestHashtablePut(t *testing.T) {
	h := hash.NewCyclicPoly(4)
	ht := newHashTable(minTableExponent, h)
	b := []byte("Hell")
	r := ht.get(b)
	if len(r) != 0 {
		t.Fatalf("len(r) %d; want 0", len(r))
	}
	ht.put(b, 42)
	r = ht.get(b)
	if len(r) != 1 {
		t.Fatalf("len(r) %d; want 1", len(r))
	}
	if r[0] != 42 {
		t.Fatalf("r[0] %d; want 42", r[0])
	}
	ht.put(b, 43)
	r = ht.get(b)
	if len(r) != 2 {
		t.Fatalf("len(r) %d; want 2", len(r))
	}
	for i := 0; i < 2; i++ {
		if r[i] != uint32(42+i) {
			t.Fatalf("r[%d] %d; want %d", i, r[i], uint32(42+i))
		}
	}
}
