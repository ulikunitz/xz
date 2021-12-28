package lzma

import (
	"bytes"
	"crypto/sha256"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"testing"
)

func TestHeader(t *testing.T) {
	tests := []params{
		{Properties{3, 0, 2}, 1 << 15, eosSize},
	}
	for _, tc := range tests {
		s, _ := tc.AppendBinary(nil)
		var p params
		if err := p.UnmarshalBinary(s); err != nil {
			t.Fatalf("h.parse error %s", err)
		}
		if p != tc {
			t.Fatalf("got %+v; want %+v", p, tc)
		}
	}
}

func TestReaderSimple(t *testing.T) {
	const file = "testdata/fox.lzma"
	const text = "The quick brown fox jumps over the lazy dog.\n"

	f, err := os.Open(file)
	if err != nil {
		t.Fatalf("os.Open(%q) error %s", file, err)
	}
	defer f.Close()

	r, err := NewReader(f)
	if err != nil {
		t.Fatalf("NewReader(f) error %s", err)
	}
	buf := new(bytes.Buffer)
	_, err = io.Copy(buf, r)
	if err != nil {
		t.Fatalf("%s: io.Copy(buf, r) error %s", file, err)
	}
	s := buf.String()
	if s != text {
		t.Fatalf("got %q; want %q", s, text)
	}

	fi, err := f.Stat()
	if err != nil {
		t.Fatalf("f.State() error %s", err)
	}

	n, err := f.Seek(0, os.SEEK_CUR)
	if err != nil {
		t.Fatalf("f.Seek() error %s", err)
	}
	if n != fi.Size() {
		t.Fatalf("f pos %d; want eof pos = %d", n, fi.Size())
	}
}

func TestGoodExamples(t *testing.T) {
	files, err := filepath.Glob("testdata/examples/a*.lzma")
	if err != nil {
		t.Fatalf("Glob error %s", err)
	}

	const textFile = "testdata/examples/a.txt"
	text, err := os.ReadFile(textFile)
	if err != nil {
		t.Fatalf("os.ReadFile(%q) error %s", textFile, err)
	}

	for _, file := range files {
		f, err := os.Open(file)
		if err != nil {
			t.Errorf("os.Open(%q) error %s", file, err)
			continue
		}

		r, err := NewReader(f)
		if err != nil {
			t.Errorf("NewReader(f) error %s", err)
			continue
		}

		buf := new(bytes.Buffer)
		_, err = io.Copy(buf, r)
		if err != nil {
			t.Errorf("io.Copy(buf, r) error %s", err)
			continue
		}
		s := buf.Bytes()
		t.Logf("got: %q", s)

		if !bytes.Equal(s, text) {
			t.Errorf("got %q; want %q", s, text)
			continue
		}

		fi, err := f.Stat()
		if err != nil {
			t.Errorf("f.State() error %s", err)
			continue
		}

		n, err := f.Seek(0, os.SEEK_CUR)
		if err != nil {
			t.Errorf("f.Seek() error %s", err)
			continue
		}
		if n != fi.Size() {
			t.Errorf("f pos %d; want eof pos = %d", n, fi.Size())
			continue
		}

	}
}

func TestBadExamples(t *testing.T) {
	files, err := filepath.Glob("testdata/examples/bad*.lzma")
	if err != nil {
		t.Fatalf("Glob error %s", err)
	}

	for i, file := range files {
		t.Run(fmt.Sprintf("%d", i+1), func(t *testing.T) {
			f, err := os.Open(file)
			if err != nil {
				t.Errorf("os.Open(%q) error %s", file, err)
				return
			}

			r, err := NewReader(f)
			if err != nil {
				t.Logf("NewReader(f) error %s", err)
				return
			}

			buf := new(bytes.Buffer)
			_, err = io.Copy(buf, r)
			if err != nil {
				t.Logf("io.Copy(buf, r) error %s", err)
				return
			}

			t.Errorf("no error for %s", file)
		})
	}
}

func TestWriterSimple(t *testing.T) {
	const text = "The quick brown fox jumps over the lazy dog.\n"
	var buf bytes.Buffer
	w, err := NewWriter(&buf)
	if err != nil {
		t.Fatalf("NewWriter error %s", err)
	}
	if _, err = io.WriteString(w, text); err != nil {
		t.Fatalf("io.WriteString(w, %q) error %s", text, err)
	}
	if err = w.Close(); err != nil {
		t.Fatalf("w.Close() error %s", err)
	}
	r, err := NewReader(&buf)
	if err != nil {
		t.Fatalf("NewReader error %s", err)
	}
	var out bytes.Buffer
	if _, err = io.Copy(&out, r); err != nil {
		t.Fatalf("io.Copy error %s", err)
	}

	g := out.String()
	if g != text {
		t.Fatalf("got %q; want %q", g, text)
	}
}
func TestWriterFile(t *testing.T) {
	const file = "testdata/enwik7"

	f, err := os.Open(file)
	if err != nil {
		t.Fatalf("os.Open(%q) error %s", file, err)
	}
	defer f.Close()
	h1 := sha256.New()
	fr := io.TeeReader(f, h1)

	var buf bytes.Buffer
	w, err := NewWriter(&buf)
	if err != nil {
		t.Fatalf("NewWriter error %s", err)
	}
	if _, err = io.Copy(w, fr); err != nil {
		t.Fatalf("io.Copy(w, r) error %s", err)
	}
	if err = w.Close(); err != nil {
		t.Fatalf("w.Close() error %s", err)
	}
	t.Logf("buf.Len() %d", buf.Len())

	r, err := NewReader(&buf)
	if err != nil {
		t.Fatalf("NewReader error %s", err)
	}
	h2 := sha256.New()
	if _, err = io.Copy(h2, r); err != nil {
		t.Fatalf("io.Copy error %s", err)
	}

	hash1 := h1.Sum(nil)
	hash2 := h2.Sum(nil)

	if !bytes.Equal(hash1, hash2) {
		t.Fatalf("go hash %x; want %x", hash2, hash1)
	}
}
