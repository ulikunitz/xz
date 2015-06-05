package i64

import "testing"

func TestAdd(t *testing.T) {
	tests := [...]struct {
		x, y, z  int64
		overflow bool
	}{
		{1, 1, 2, false},
		{Max, 1, Min, true},
		{Max - 2, 3, Min, true},
		{1, Max, Min, true},
		{1, Min, Min + 1, false},
		{Min, -1, Max, true},
		{-1, Min, Max, true},
	}
	for _, c := range tests {
		z, overflow := Add(c.x, c.y)
		if z != c.z {
			t.Errorf("%#x + %#x = %#x; want %#x", c.x, c.y, z, c.z)
		}
		if overflow != c.overflow {
			t.Errorf("%#x + %#x = %t; want %t", c.x, c.y,
				overflow, c.overflow)
		}
	}
}

func TestSub(t *testing.T) {
	tests := [...]struct {
		x, y, z  int64
		overflow bool
	}{
		{1, 1, 0, false},
		{Max, -1, Min, true},
		{1, Max, Min + 2, false},
		{1, Min, Min + 1, true},
		{Min, -1, Min + 1, false},
		{Min, 1, Max, true},
		{-1, Min, Max, false},
		{Max - 2, -3, Min, true},
	}
	for _, c := range tests {
		z, overflow := Sub(c.x, c.y)
		if z != c.z {
			t.Errorf("%#x - %#x = %#x; want %#x", c.x, c.y, z, c.z)
		}
		if overflow != c.overflow {
			t.Errorf("%#x - %#x = %t; want %t", c.x, c.y,
				overflow, c.overflow)
		}
	}
}
