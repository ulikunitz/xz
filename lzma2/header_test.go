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
