package lzma

import (
	"bytes"
	"io/ioutil"
	"os"
	"testing"
)

func TestNewDecoder(t *testing.T) {
	f, err := os.Open("examples/a.lzma")
	if err != nil {
		t.Fatalf("open examples/a.lzma: %s", err)
	}
	defer f.Close()
	d, err := NewDecoder(f)
	if err != nil {
		t.Fatalf("NewDecoder: %s", err)
	}
	t.Logf("decoder %#v", d)
	if d.properties.LC != 3 {
		t.Errorf("LC %d; want %d", d.properties.LC, 3)
	}
	if d.properties.LP != 0 {
		t.Errorf("LP %d; want %d", d.properties.LP, 0)
	}
	if d.properties.PB != 2 {
		t.Errorf("PB %d; want %d", d.properties.PB, 2)
	}
}

func TestDecoderSimple(t *testing.T) {
	tst = t
	f, err := os.Open("examples/a.lzma")
	if err != nil {
		t.Fatalf("open examples/a.lzma: %s", err)
	}
	defer f.Close()
	d, err := NewDecoder(f)
	if err != nil {
		t.Fatalf("NewDecoder: %s", err)
	}
	t.Logf("unpackLen %d", d.unpackLen)
	decompressed, err := ioutil.ReadAll(d)
	if err != nil {
		t.Fatalf("ReadAll: %s", err)
	}
	t.Logf("%s", decompressed)
	orig, err := ioutil.ReadFile("examples/a.txt")
	if err != nil {
		t.Fatalf("ReadFile: %s", err)
	}
	if len(orig) != len(decompressed) {
		t.Fatalf("length decompressed is %d; want %d",
			len(decompressed), len(orig))
	}
	if !bytes.Equal(orig, decompressed) {
		t.Fatalf("decompressed file differs from original")
	}
}
