package lzma

import (
	"fmt"
	"testing"
)

func peek(d *DecoderDict) []byte {
	p := make([]byte, d.buffered())
	k, err := d.peek(p)
	if err != nil {
		panic(fmt.Errorf("peek: "+
			"Read returned unexpected error %s", err))
	}
	if k != len(p) {
		panic(fmt.Errorf("peek: "+
			"Read returned %d; wanted %d", k, len(p)))
	}
	return p
}

func TestNewDecoderDict(t *testing.T) {
	if _, err := NewDecoderDict(0); err == nil {
		t.Fatalf("no error for zero dictionary capacity")
	}
	if _, err := NewDecoderDict(8); err != nil {
		t.Fatalf("error %s", err)
	}
}
