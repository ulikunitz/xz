package lzma

import (
	"bytes"
	"math/rand"
	"testing"
)

func random(lc, lp uint) (s byte, state uint32, match byte, litState uint32) {
	s = byte(rand.Int31n(256))
	state = uint32(rand.Int31n(maxState + 1))
	match = byte(rand.Int31n(256))
	litState = uint32(rand.Int31n(1<<lp)<<lc | rand.Int31n(1<<lc))
	return
}

func TestLiteralCodec(t *testing.T) {
	const (
		lc = 3
		lp = 1
	)
	const count = 1000
	var err error
	var buf bytes.Buffer
	e := newRangeEncoder(&buf)
	le := newLiteralCodec(lc, lp)
	rand.Seed(1)
	for i := 0; i < count; i++ {
		s, state, match, litState := random(lc, lp)
		if err = le.Encode(s, e, state, match, litState); err != nil {
			t.Fatalf("le.Encode: %s", err)
		}
	}
	if err = e.Flush(); err != nil {
		t.Fatalf("e.Flush: %s", err)
	}
	t.Logf("buffer length %d", buf.Len())
	d, err := newRangeDecoder(&buf)
	if err != nil {
		t.Fatalf("newRangeDecoder: %s", err)
	}
	ld := newLiteralCodec(lc, lp)
	rand.Seed(1)
	for i := 0; i < count; i++ {
		s, state, match, litState := random(lc, lp)
		r, err := ld.Decode(d, state, match, litState)
		if err != nil {
			t.Fatalf("ld.Decode: %s", err)
		}
		if r != s {
			t.Fatalf("ld.Decode: %#2x; want %#2x", r, s)
		}
	}
}
