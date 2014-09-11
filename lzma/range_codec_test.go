package lzma

import (
	"bytes"
	"testing"
)

var testStrings = []string{
	"S",
	"HalloBallo",
	"funny",
	"Die Nummer Eins der Welt sind wir!",
}

func TestDirectEncoding(t *testing.T) {
	for _, s := range testStrings {
		var buf bytes.Buffer
		e := newRangeEncoder(&buf)
		de := makeDirectEncoder(8)
		b := []byte(s)
		for _, x := range b {
			if err := de.Encode(uint32(x), e); err != nil {
				t.Fatalf("de.Encode: %s", err)
			}
		}
		if err := e.Flush(); err != nil {
			t.Fatalf("e.Flush: %s", err)
		}
		var out []byte
		d, err := newRangeDecoder(&buf)
		if err != nil {
			t.Fatalf("newRangeDecoder: %s", err)
		}
		dd := makeDirectDecoder(8)
		for i := 0; i < len(b); i++ {
			x, err := dd.Decode(d)
			if err != nil {
				t.Fatalf("dd.Decode: %s", err)
			}
			out = append(out, byte(x))
		}
		if !d.FinishingOk() {
			t.Fatal("finishing not ok")
		}
		if !bytes.Equal(out, b) {
			t.Errorf("error %q; want %q", out, b)
		}
	}
}

func TestTreeEncoding(t *testing.T) {
	for _, s := range testStrings {
		var buf bytes.Buffer
		e := newRangeEncoder(&buf)
		te := makeTreeEncoder(8)
		b := []byte(s)
		for _, x := range b {
			if err := te.Encode(uint32(x), e); err != nil {
				t.Fatalf("te.Encode: %s", err)
			}
		}
		if err := e.Flush(); err != nil {
			t.Fatalf("e.flush: %s", err)
		}
		var out []byte
		d, err := newRangeDecoder(&buf)
		if err != nil {
			t.Fatalf("newRangeDecoder: %s", err)
		}
		td := makeTreeDecoder(8)
		for i := 0; i < len(b); i++ {
			x, err := td.Decode(d)
			if err != nil {
				t.Fatalf("td.Decode: %s", err)
			}
			out = append(out, byte(x))
		}
		if !bytes.Equal(out, b) {
			t.Errorf("error %q; want %q", out, b)
		}
	}
}

func TestTreeReverseEncoding(t *testing.T) {
	for _, s := range testStrings {
		var buf bytes.Buffer
		e := newRangeEncoder(&buf)
		te := makeTreeReverseEncoder(8)
		b := []byte(s)
		for _, x := range b {
			if err := te.Encode(uint32(x), e); err != nil {
				t.Fatalf("te.Encode: %s", err)
			}
		}
		if err := e.Flush(); err != nil {
			t.Fatalf("e.flush: %s", err)
		}
		var out []byte
		d, err := newRangeDecoder(&buf)
		if err != nil {
			t.Fatalf("newRangeDecoder: %s", err)
		}
		td := makeTreeReverseDecoder(8)
		for i := 0; i < len(b); i++ {
			x, err := td.Decode(d)
			if err != nil {
				t.Fatalf("td.Decode: %s", err)
			}
			out = append(out, byte(x))
		}
		if !bytes.Equal(out, b) {
			t.Errorf("error %q; want %q", out, b)
		}
	}
}
