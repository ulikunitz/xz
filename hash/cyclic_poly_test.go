package hash

import "testing"

func TestCyclicPolySimple(t *testing.T) {
	p := []byte("abcde")
	r := NewCyclicPoly(4)
	h2 := r.Hashes(p)
	for i, h := range h2 {
		w := r.Hashes(p[i : i+4])[0]
		t.Logf("%d h=%#016x w=%#016x", i, h, w)
		if h != w {
			t.Errorf("rolling hash %d: %#016x; want %#016x",
				i, h, w)
		}
	}
}

func BenchmarkCyclicPoly(b *testing.B) {
	p := makeBenchmarkBytes(4096)
	r := NewCyclicPoly(4)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		r.Hashes(p)
	}
}
