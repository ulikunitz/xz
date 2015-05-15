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
		t.Fatalf("newDict error %s", err)
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
		t.Fatalf("newDict error %s", err)
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
		{-1, errOffset},
		{20, errOffset},
		{4, nil},
	}
	for _, c := range tests {
		err := d.move(c.n)
		if err != c.err {
			t.Errorf("d.move(%d) returned error %s; expected %s",
				c.n, err, c.err)
		}
	}
}

func TestDict_Seek(t *testing.T) {
	d := someDict(t)
	tests := []struct {
		offset int64
		whence int
		off    int64
		err    error
	}{
		{0, 0, 0, nil},
		{2, 1, 2, nil},
		{0, 2, 8, nil},
		{0, 3, 8, errWhence},
		{-1, 0, 8, errOffset},
		{9, 0, 8, errOffset},
	}
	for _, c := range tests {
		off, err := d.Seek(c.offset, c.whence)
		if err != c.err {
			t.Errorf("d.Seek(%d, %d) error %s; want %s",
				c.offset, c.whence, err, c.err)
		}
		if off != c.off {
			t.Errorf("d.Seek(%d, %d) off %d; want %d",
				c.offset, c.whence, off, c.off)
		}
	}
}
