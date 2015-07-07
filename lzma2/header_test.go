package lzma2

import (
	"fmt"
	"testing"
)

func TestChunkTypeString(t *testing.T) {
	tests := [...]struct {
		c chunkType
		s string
	}{
		{cEOS, "EOS"},
		{cU, "U"},
		{cUD, "UD"},
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
