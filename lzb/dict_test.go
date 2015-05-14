package lzb

import "testing"

func TestNewDict(t *testing.T) {
	b := newBuffer(10)
	b.Write(fillBytes(8))
	d, err := newDict(b, 4, 0)
	if err == nil {
		t.Fatalf("newDict returned no error for size zero")
	}
	if d != nil {
		t.Fatalf("newDict returns non-nil dictionary after error")
	}
	d, err = newDict(b, 4, 11)
	if err == nil {
		t.Fatalf("newDict returned no error for a size exceeding " +
			"the buffer capacity")
	}
	if d != nil {
		t.Fatalf("newDict returns non-nil dictionary after error")
	}
	d, err = newDict(b, 9, 10)
	if err == nil {
		t.Fatalf("newDict returned no error for " +
			"out-of-range head offset")
	}
	if d != nil {
		t.Fatalf("newDict returns non-nil dictionary after error")
	}
	d, err = newDict(b, 8, 10)
	if err != nil {
		t.Fatalf("newDict error %s")
	}
	if d == nil {
		t.Fatalf("successful newDict returns nil dictionary pointer")
	}
}

func someDict(t *testing.T) *dict {
	b := newBuffer(10)
	b.Write(fillBytes(8))
	d, err := newDict(b, 8, 10)
	if err != nil {
		t.Fatalf("newDict error %s")
	}
	return d
}

func TestDict_byteAt(t *testing.T) {
	d := someDict(t)
	func() {
		defer func() {
			if r := recover(); r == nil {
				t.Fatalf("no panic for dist too large")
			}
		}()
		d.byteAt(11)
	}()
	func() {
		defer func() {
			if r := recover(); r == nil {
				t.Fatalf("no panic on dist too small")
			}
		}()
		d.byteAt(0)
	}()
	tests := []struct {
		dist int64
		b    byte
	}{
		{1, 7},
		{2, 6},
		{7, 1},
		{8, 0},
		{9, 0},
		{10, 0},
	}
	for _, c := range tests {
		b := d.byteAt(c.dist)
		if b != c.b {
			t.Fatalf("d.byteAt(%d) returned %#02x; want %#02x",
				c.dist, b, c.b)
		}
	}
}

func TestDict_move(t *testing.T) {
	d := someDict(t)
	d.head = 0
	tests := []struct {
		n   int
		err error
	}{
		{-1, errMove},
		{20, errMove},
		{4, nil},
	}
	for _, c := range tests {
		err := d.move(c.n)
		if err != c.err {
			t.Errorf("d.move(%d) returned error %s; expected %s",
				err, c.err)
		}
	}
}
