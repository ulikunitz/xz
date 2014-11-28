package hash

import "testing"

func TestRabinKarpSimple(t *testing.T) {
	p := []byte("abcde")
	r := NewRabinKarp(4)
	h2 := ComputeHashes(r, p)
	for i, h := range h2 {
		w := ComputeHashes(r, p[i:i+4])[0]
		t.Logf("%d h=%#016x w=%#016x", i, h, w)
		if h != w {
			t.Errorf("rolling hash %d: %#016x; want %#016x",
				i, h, w)
		}
	}
}
