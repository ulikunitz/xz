package lzb

import (
	"fmt"
	"testing"
)

func TestNewReadBuffer(t *testing.T) {
	r, err := newReadBuffer(10, 10)
	if err != nil {
		t.Fatalf("newReadBuffer error %s", err)
	}
	c := r.capacity()
	if c != 10 {
		t.Errorf("capacity is %d; want %d", c, 10)
	}
	s := r.dict.size
	if s != 10 {
		t.Errorf("dict size is %d; want %d", s, 10)
	}
}

func mustNewReadBuffer(capacity, histsize int64) *readBuffer {
	b, err := newReadBuffer(capacity, histsize)
	if err != nil {
		panic(fmt.Errorf("newReadBuffer error %s", err))
	}
	return b
}

func TestReadBuffer_Seek(t *testing.T) {
	r := mustNewReadBuffer(10, 10)
	p := []byte("abcdef")
	n, err := r.Write(p)
	if err != nil {
		t.Fatalf("r.Write(%q) error %s", p, err)
	}
	if n != len(p) {
		t.Fatalf("r.Write(%q) returned %d; want %d", p, n, len(p))
	}
	tests := []struct {
		offset int64
		whence int
		off    int64
		err    error
	}{
		{2, 0, 2, nil},
		{1, 1, 3, nil},
		{-1, 2, 5, nil},
		{0, 0, 0, nil},
		{-1, 0, 0, errOffset},
		{6, 0, 6, nil},
		{7, 0, 6, errOffset},
		{5, 3, 6, errWhence},
	}
	for _, c := range tests {
		off, err := r.Seek(c.offset, c.whence)
		if err != c.err {
			t.Errorf("r.Seek(%d, %d) returned error %s; want %s",
				c.offset, c.whence, err, c.err)
		}
		if off != c.off {
			t.Errorf("r.Seek(%d, %d) returned offset %d; want %d",
				c.offset, c.whence, off, c.off)
		}
	}
}
