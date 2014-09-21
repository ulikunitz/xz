package lzma

import "testing"

func TestNtz32(t *testing.T) {
	for i := 0; i < 31; i++ {
		x := uint32(1) << uint(i)
		k := ntz32(x)
		if k != i {
			t.Errorf("ntz32(%#08x) = %d; want %d", x, k, i)
		}
		x = uint32(0xffffffef) << uint(i)
		k = ntz32(x)
		if k != i {
			t.Errorf("ntz32(%#08x) = %d; want %d", x, k, i)
		}
	}
}
