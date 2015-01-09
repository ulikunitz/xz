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
	for l := uint32(0); l < maxLength-minLength; l++ {
		if err = le.Encode(e, l, 0); err != nil {
			t.Fatalf("le.Encode: %s", err)
		}
	}
	if err = e.Close(); err != nil {
		t.Fatalf("e.Close(): %s", err)
	}
	t.Logf("buffer length: %d", buf.Len())
	d, err := newRangeDecoder(&buf)
	if err != nil {
		t.Fatalf("newRangeDecoder: %s", err)
	}
	ld := newLengthCodec()
	for l := uint32(0); l < maxLength-minLength; l++ {
		x, err := ld.Decode(d, 0)
		if err != nil {
			t.Fatalf("ld.Decode: %s", err)
		}
		if x != l {
			t.Fatalf("ld.Decode: got %d; want %d", x, l)
		}
	}
}

func TestLengthCodecRange(t *testing.T) {
	var buf bytes.Buffer
	e := newRangeEncoder(&buf)
	le := newLengthCodec()
	l := uint32(maxLength - minLength + 1)
	err := le.Encode(e, l, 0)
	if err == nil {
		t.Fatalf("le.Encode: no error for length %d", l)
	}
	t.Logf("error %s", err)
}

func TestLengthCodecAll(t *testing.T) {
	var buf bytes.Buffer
	e := newRangeEncoder(&buf)
	le := newLengthCodec()
	for i := minLength; i < maxLength; i++ {
		u := uint32(i - minLength)
		err := le.Encode(e, u, 0)
		if err != nil {
			t.Fatalf("le.Encode(e, %d, 0) error %s", u, err)
		}
	}
	if err := e.Close(); err != nil {
		t.Fatalf("e.Close error %s", err)
	}
	d, err := newRangeDecoder(&buf)
	if err != nil {
		t.Fatalf("newRangeDecoder error %s", err)
	}
	ld := newLengthCodec()
	for i := minLength; i < maxLength; i++ {
		u := uint32(i - minLength)
		l, err := ld.Decode(d, 0)
		if err != nil {
			t.Fatalf("ld.Decode(e, 0) error %s", err)
		}
		if l != u {
			t.Errorf("ld.Decode(e, 0) returned %d; want %d", l, u)
		}
	}
}
