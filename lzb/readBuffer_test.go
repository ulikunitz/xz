package lzb

import "testing"

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
