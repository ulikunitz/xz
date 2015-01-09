package lzma

import (
	"bytes"
	"io"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
	"testing"
	"testing/iotest"
)

func TestNewReader(t *testing.T) {
	f, err := os.Open("examples/a.lzma")
	if err != nil {
		t.Fatalf("open examples/a.lzma: %s", err)
	}
	defer f.Close()
	l, err := NewReader(f)
	if err != nil {
		t.Fatalf("NewReader: %s", err)
	}
	props := l.Properties()
	if props.LC != 3 {
		t.Errorf("LC %d; want %d", props.LC, 3)
	}
	if props.LP != 0 {
		t.Errorf("LP %d; want %d", props.LP, 0)
	}
	if props.PB != 2 {
		t.Errorf("PB %d; want %d", props.PB, 2)
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
	l, err := NewReader(f)
	if err != nil {
		t.Fatalf("NewReader: %s", err)
	}
	decoded, err := ioutil.ReadAll(l)
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

func TestReaderSimple(t *testing.T) {
	// DebugOn(os.Stderr)
	// defer DebugOff()

	testDecodeFile(t, "a.lzma", readOrigFile(t))
}

func TestReaderAll(t *testing.T) {
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

//
func Example_reader() {
	f, err := os.Open("fox.lzma")
	if err != nil {
		log.Fatal(err)
	}
	r, err := NewReader(f)
	if err != nil {
		log.Fatal(err)
	}
	_, err = io.Copy(os.Stdout, r)
	if err != nil {
		log.Fatal(err)
	}
	// Output:
	// The quick brown fox jumps over the lazy dog.
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
	l, err := NewReader(w.wrap(f))
	if err != nil {
		t.Fatalf("NewReader: %s", err)
	}
	decoded, err := ioutil.ReadAll(l)
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

func TestReaderWrap(t *testing.T) {
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

func TestReaderBadFiles(t *testing.T) {
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
	// filter now all file with the pattern "bad*.lzma"
	files := make([]string, 0, len(all))
	for _, fn := range all {
		match, err := filepath.Match("bad*.lzma", fn)
		if err != nil {
			t.Fatalf("Match: %s", err)
		}
		if match {
			files = append(files, fn)
		}
	}
	t.Log("files:", files)
	for _, filename := range files {
		pathname := filepath.Join(dirname, filename)
		f, err := os.Open(pathname)
		if err != nil {
			t.Fatalf("Open(\"%s\"): %s", pathname, err)
		}
		defer f.Close()
		t.Logf("file %s opened", filename)
		l, err := NewReader(f)
		if err != nil {
			t.Fatalf("NewReader: %s", err)
		}
		decoded, err := ioutil.ReadAll(l)
		if err == nil {
			t.Errorf("ReadAll for %s: no error", filename)
			t.Logf("%s", decoded)
			continue
		}
		t.Logf("%s: error %s", filename, err)
	}
}
