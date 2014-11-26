package lzma

import (
	"bytes"
	"io/ioutil"
	"os"
	"path/filepath"
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
	// DebugOn(os.Stderr)
	// defer DebugOff()

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

func TestDecoderAll(t *testing.T) {
	dirname := "examples"
	dir, err := os.Open(dirname)
	if err != nil {
		t.Fatalf("Open: %s", err)
	}
	defer dir.Close()
	all, err := dir.Readdirnames(0)
	if err != nil {
		t.Fatalf("Readdirnames: %s", err)
	}
	files := make([]string, 0, len(all))
	for _, fn := range all {
		match, err := filepath.Match("a*.lzma", fn)
		if err != nil {
			t.Fatalf("Match: %s", err)
		}
		if match {
			files = append(files, fn)
		}
	}
	t.Log("files:", files)
	origFn := filepath.Join(dirname, "a.txt")
	orig, err := ioutil.ReadFile(origFn)
	if err != nil {
		t.Fatalf("ReadFile(\"%s\"): %s", origFn, err)
	}
	for _, fn := range files {
		pn := filepath.Join(dirname, fn)
		f, err := os.Open(pn)
		if err != nil {
			t.Fatalf("Open(\"%s\"): %s", pn, err)
		}
		defer f.Close()
		t.Logf("file %s opened", fn)
		d, err := NewDecoder(f)
		if err != nil {
			t.Fatalf("NewDecoder: %s", err)
		}
		decompressed, err := ioutil.ReadAll(d)
		if err != nil {
			t.Fatalf("ReadAll: %s", err)
		}
		t.Logf("uncompressed:\n%s", decompressed)
		if len(orig) != len(decompressed) {
			t.Fatalf("length decompressed is %d; want %d",
				len(decompressed), len(orig))
		}
		if !bytes.Equal(orig, decompressed) {
			t.Fatalf("decompressed file differs from original")
		}
	}
}
