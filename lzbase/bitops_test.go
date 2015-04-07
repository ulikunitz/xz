package lzbase

import "testing"

func TestNTZ32(t *testing.T) {
	for i := 0; i <= 32; i++ {
		x := uint32(1) << uint(i)
		k := NTZ32(x)
		if k != i {
			t.Errorf("NTZ32(%#08x) = %d; want %d", x, k, i)
		}
		x = uint32(0xffffffef) << uint(i)
		k = NTZ32(x)
		if k != i {
			t.Errorf("NTZ32(%#08x) = %d; want %d", x, k, i)
		}
	}
}

func TestNLZ32(t *testing.T) {
	for i := 0; i <= 32; i++ {
		x := uint32(0x80000000) >> uint(i)
		k := NLZ32(x)
		if k != i {
			t.Errorf("NLZ32(%#08x) = %d; want %d", x, k, i)
		}
		x = uint32(0xefffffef) >> uint(i)
		k = NLZ32(x)
		if k != i {
			t.Errorf("NLZ32(%#08x) = %d; want %d", x, k, i)
		}
	}
}
