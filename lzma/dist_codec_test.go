package lzma

import (
	"bytes"
	"math/rand"
	"testing"
)

func randomDistL(i int) (dist, l uint32) {
	switch {
	case i < startPosModel:
		dist = uint32(i)
	case i <= maxPosSlot:
		posSlot := uint32(i)
		bits := (posSlot >> 1) - 1
		dist = (2 | (posSlot & 1)) << bits
		dist |= rand.Uint32() & ((1 << bits) - 1)
	default:
		dist = rand.Uint32()
	}
	l = uint32(rand.Int31n(273))
	return
}

func TestDistCodec(t *testing.T) {
	const count = 500
	var err error
	var buf bytes.Buffer
	e := newRangeEncoder(&buf)
	de := newDistCodec()
	rand.Seed(1)
	for i := 0; i < count; i++ {
		dist, l := randomDistL(i)
		if err = de.Encode(e, dist, l); err != nil {
			t.Fatalf("de.Encode: %s", err)
		}
	}
	if err = e.Flush(); err != nil {
		t.Fatalf("e.Flush: %s", err)
	}
	t.Logf("buffer length %d", buf.Len())

	d, err := newRangeDecoder(&buf)
	if err != nil {
		t.Fatalf("newRangeEncoder: %s", err)
	}
	dd := newDistCodec()
	rand.Seed(1)
	for i := 0; i < count; i++ {
		want, l := randomDistL(i)
		dist, err := dd.Decode(d, l)
		if err != nil {
			t.Fatalf("dd.Decode: %s", err)
		}
		if dist != want {
			t.Fatalf("#%d dd.Decode(%d, d): %#x, want %#x", i, l,
				dist, want)
		}
	}
}
