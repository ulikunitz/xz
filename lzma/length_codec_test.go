package lzma

import (
	"bytes"
	"testing"
)

func TestLengthCodec(t *testing.T) {
	var err error
	var buf bytes.Buffer
	e := newRangeEncoder(&buf)
	le := newLengthCodec()
	for l := uint32(minLength); l < maxLength; l++ {
		if err = le.Encode(l, e, 0); err != nil {
			t.Fatalf("le.Encode: %s", err)
		}
	}
	if err = e.Flush(); err != nil {
		t.Fatalf("e.Close(): %s", err)
	}
	d, err := newRangeDecoder(&buf)
	if err != nil {
		t.Fatalf("newRangeDecoder: %s", err)
	}
	ld := newLengthCodec()
	for l := uint32(minLength); l < maxLength; l++ {
		x, err := ld.Decode(d, 0)
		if err != nil {
			t.Fatalf("ld.Decode: %s", err)
		}
		if x != l {
			t.Fatalf("ld.Decode: got %d; want %d", x, l)
		}
	}
}
