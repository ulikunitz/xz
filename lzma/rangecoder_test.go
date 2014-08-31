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
		t.Log(s)
		var buf bytes.Buffer
		e := newRangeEncoder(&buf)
		b := []byte(s)
		for _, x := range b {
			if err := e.directEncode(uint32(x), 8); err != nil {
				t.Fatalf("e.directEncode: %s", err)
			}
		}
		if err := e.flush(); err != nil {
			t.Fatalf("e.flush: %s", err)
		}
		var out []byte
		d := newRangeDecoder(&buf)
		if err := d.init(); err != nil {
			t.Fatalf("d.init: %s", err)
		}
		for i := 0; i < len(b); i++ {
			x, err := d.directDecode(8)
			if err != nil {
				t.Fatalf("d.directDecode: %s", err)
			}
			out = append(out, byte(x))
		}
		if !bytes.Equal(out, b) {
			t.Errorf("error %q; want %q", out, b)
		}
	}
}

func TestTreeEncoding(t *testing.T) {
	for _, s := range testStrings {
		t.Log(s)
		var buf bytes.Buffer
		e := newRangeEncoder(&buf)
		tree := makeProbTree(8)
		b := []byte(s)
		for _, x := range b {
			if err := e.treeEncode(uint32(x), &tree); err != nil {
				t.Fatalf("e.treeEncode: %s", err)
			}
		}
		if err := e.flush(); err != nil {
			t.Fatalf("e.flush: %s", err)
		}
		var out []byte
		d := newRangeDecoder(&buf)
		if err := d.init(); err != nil {
			t.Fatalf("d.init: %s", err)
		}
		tree = makeProbTree(8)
		for i := 0; i < len(b); i++ {
			x, err := d.treeDecode(&tree)
			if err != nil {
				t.Fatalf("d.treeDecode: %s", err)
			}
			out = append(out, byte(x))
		}
		if !bytes.Equal(out, b) {
			t.Errorf("error %q; want %q", out, b)
		}
	}
}
