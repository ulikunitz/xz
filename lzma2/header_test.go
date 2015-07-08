package lzma2

import (
	"fmt"
	"testing"

	"github.com/uli-go/xz/lzma"
)

func TestChunkTypeString(t *testing.T) {
	tests := [...]struct {
		c chunkType
		s string
	}{
		{cEOS, "EOS"},
		{cUD, "UD"},
		{cU, "U"},
		{cL, "L"},
		{cLR, "LR"},
		{cLRN, "LRN"},
		{cLRND, "LRND"},
	}
	for _, c := range tests {
		s := fmt.Sprintf("%v", c.c)
		if s != c.s {
			t.Errorf("got %s; want %s", s, c.s)
		}
	}
}

func TestHeaderChunkType(t *testing.T) {
	tests := []struct {
		h byte
		c chunkType
	}{
		{h: 0, c: cEOS},
		{h: 1, c: cUD},
		{h: 2, c: cU},
		{h: 1<<7 | 0x1f, c: cL},
		{h: 1<<7 | 1<<5 | 0x1f, c: cLR},
		{h: 1<<7 | 1<<6 | 0x1f, c: cLRN},
		{h: 1<<7 | 1<<6 | 1<<5 | 0x1f, c: cLRND},
		{h: 1<<7 | 1<<6 | 1<<5, c: cLRND},
	}
	if _, err := headerChunkType(3); err == nil {
		t.Fatalf("headerChunkType(%d) got %v; want %v",
			3, err, errHeaderByte)
	}
	for _, tc := range tests {
		c, err := headerChunkType(tc.h)
		if err != nil {
			t.Fatalf("headerChunkType error %s", err)
		}
		if c != tc.c {
			t.Errorf("got %s; want %s", c, tc.c)
		}
	}
}

func TestHeaderLen(t *testing.T) {
	tests := []struct {
		c chunkType
		n int
	}{
		{cEOS, 1}, {cU, 3}, {cUD, 3}, {cL, 5}, {cLR, 5}, {cLRN, 6},
		{cLRND, 6},
	}
	for _, tc := range tests {
		n := headerLen(tc.c)
		if n != tc.n {
			t.Errorf("header length for %s %d; want %d",
				tc.c, n, tc.n)
		}
	}
}

func TestMarshalling(t *testing.T) {
	props, err := lzma.NewProperties(3, 0, 2)
	if err != nil {
		t.Fatalf("NewProperties(3, 0, 2) error %s", err)
	}

	var h, g chunkHeader
	for c := cEOS; c <= cLRND; c++ {
		h.ctype = c
		if c >= cUD {
			h.unpacked = 0x0304
		}
		if c >= cL {
			h.packed = 0x0201
		}
		if c >= cLRN {
			h.props = props
		}
		data, err := h.MarshalBinary()
		if err != nil {
			t.Fatalf("MarshalBinary for %v error %s", &h, err)
		}
		if err = g.UnmarshalBinary(data); err != nil {
			t.Fatalf("UnmarshalBinary error %s", err)
		}
		if g != h {
			t.Fatalf("got %v; want %v", g, h)
		}
	}
}
