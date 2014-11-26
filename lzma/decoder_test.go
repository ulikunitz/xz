package lzma

import (
	"bytes"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"
	"testing/iotest"
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

const (
	dirname  = "examples"
	origname = "a.txt"
)

func readOrigFile(t *testing.T) []byte {
	orig, err := ioutil.ReadFile(filepath.Join(dirname, origname))
	if err != nil {
		t.Fatalf("ReadFile: %s", err)
	}
	return orig
}

func testDecodeFile(t *testing.T, filename string, orig []byte) {
	pathname := filepath.Join(dirname, filename)
	f, err := os.Open(pathname)
	if err != nil {
		t.Fatalf("Open(\"%s\"): %s", pathname, err)
	}
	defer f.Close()
	t.Logf("file %s opened", filename)
	d, err := NewDecoder(f)
	if err != nil {
		t.Fatalf("NewDecoder: %s", err)
	}
	t.Logf("unpackLen %d", d.unpackLen)
	decoded, err := ioutil.ReadAll(d)
	if err != nil {
		t.Fatalf("ReadAll: %s", err)
	}
	t.Logf("%s", decoded)
	if len(orig) != len(decoded) {
		t.Fatalf("length decoded is %d; want %d",
			len(decoded), len(orig))
	}
	if !bytes.Equal(orig, decoded) {
		t.Fatalf("decoded file differs from original")
	}
}

func TestDecoderSimple(t *testing.T) {
	// DebugOn(os.Stderr)
	// defer DebugOff()

	testDecodeFile(t, "a.lzma", readOrigFile(t))
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
	// filter now all file with the pattern "a*.lzma"
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
	orig := readOrigFile(t)
	// actually test the files
	for _, fn := range files {
		testDecodeFile(t, fn, orig)
	}
}

type wrapTest struct {
	name string
	wrap func(io.Reader) io.Reader
}

func (w *wrapTest) testFile(t *testing.T, filename string, orig []byte) {
	pathname := filepath.Join(dirname, filename)
	f, err := os.Open(pathname)
	if err != nil {
		t.Fatalf("Open(\"%s\"): %s", pathname, err)
	}
	defer f.Close()
	t.Logf("%s file %s opened", w.name, filename)
	d, err := NewDecoder(w.wrap(f))
	if err != nil {
		t.Fatalf("NewDecoder: %s", err)
	}
	t.Logf("unpackLen %d", d.unpackLen)
	decoded, err := ioutil.ReadAll(d)
	if err != nil {
		t.Fatalf("%s ReadAll: %s", w.name, err)
	}
	t.Logf("%s", decoded)
	if len(orig) != len(decoded) {
		t.Fatalf("%s length decoded is %d; want %d",
			w.name, len(decoded), len(orig))
	}
	if !bytes.Equal(orig, decoded) {
		t.Fatalf("%s decoded file differs from original", w.name)
	}
}

func TestDecoderWrap(t *testing.T) {
	tests := [...]wrapTest{
		{"DataErrReader", iotest.DataErrReader},
		{"HalfReader", iotest.HalfReader},
		{"OneByteReader", iotest.OneByteReader},
		{"TimeoutReader", iotest.TimeoutReader},
	}
	orig := readOrigFile(t)
	for _, tst := range tests {
		tst.testFile(t, "a.lzma", orig)
	}
}
