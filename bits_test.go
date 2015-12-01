package xz

import "testing"

func TestVariableLengthEncoding(t *testing.T) {
	tests := []uint64{0, 0x80, 0x100, 1<<64 - 1}
	p := make([]byte, 10)
	for _, u := range tests {
		p = p[:10]
		n, err := encodeU64(p, u)
		if err != nil {
			t.Errorf("encodeU64(p, %#x): %d, %s", u, n, err)
		}
		v, k, err := decodeU64(p)
		if err != nil {
			t.Errorf("decodeU64(p) for %#x: %#x, %d, %s",
				u, k, v, err)
		}
		if v != u {
			t.Errorf("decodeU64(p) returned %#x; want %#x",
				v, u)
		}
		if k != n {
			t.Errorf("decodeU64(p) for %#x returned length %d;"+
				" want %d", u, k, n)
		}
	}
}
