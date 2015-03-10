package lzma2

import "testing"

func TestDictSizeString(t *testing.T) {
	for s := DictSize(0); s <= 40; s++ {
		t.Logf("%d %s", s, s)
	}
}

func TestDictSizeCeil(t *testing.T) {
	for s := DictSize(0); s < 41; s++ {
		m := s.Size()
		r := DictSizeCeil(m)
		if r != s {
			t.Fatalf("dictSize for %s: %d; want %d", s, r, s)
		}
		if s == 40 {
			break
		}
		r = DictSizeCeil(m + 1)
		if r != s+1 {
			t.Fatalf("dictSize for size %d: %d; want %d", m+1, r,
				s+1)
		}
	}
}
