package lzma

import (
	"bytes"
	"crypto/sha256"
	"fmt"
	"io"
	"os"
	"strings"
	"testing"
)

func TestWriter2Simple(t *testing.T) {
	const s = "=====foofoobar==foobar===="

	buf := new(bytes.Buffer)
	w, err := NewWriter2(buf)
	if err != nil {
		t.Fatalf("NewWriter2(buf) error %s", err)
	}
	dictSize := w.DictSize()
	t.Logf("dictSize: %d", dictSize)

	if _, err = io.WriteString(w, s); err != nil {
		t.Fatalf("io.WriteString(w, %q) error %s", s, err)
	}

	if err = w.Close(); err != nil {
		t.Fatalf("w.Close() error %s", err)
	}

	t.Logf("buf.Len() %d; len(s) %d", buf.Len(), len(s))

	r, err := NewReader2(buf, dictSize)
	if err != nil {
		t.Fatalf("NewReader2(buf) error %s", err)
	}

	sb := new(strings.Builder)
	if _, err = io.Copy(sb, r); err != nil {
		t.Fatalf("io.Copy(sb, r) error %s", err)
	}

	g := sb.String()
	if g != s {
		t.Fatalf("got %q; want %q", g, s)
	}
}

func TestWriter2(t *testing.T) {
	tests := []Writer2Config{
		{Workers: 1},
		{WorkerBufferSize: 100000, Workers: 2},
		{WorkerBufferSize: 3e5},
		{},
	}

	for i, cfg := range tests {
		cfg := cfg
		t.Run(fmt.Sprintf("%d", i+1), func(t *testing.T) {
			const file = "testdata/enwik7"
			f, err := os.Open(file)
			if err != nil {
				t.Fatalf("os.Open(%q) error %s", file, err)
			}
			defer f.Close()

			h1 := sha256.New()

			buf := new(bytes.Buffer)
			w, err := NewWriter2Config(buf, cfg)
			if err != nil {
				t.Fatalf("NewWriter2Config error %s", err)
			}
			defer w.Close()
			dictSize := w.DictSize()
			t.Logf("dictSize: %d", dictSize)

			n1, err := io.Copy(w, io.TeeReader(f, h1))
			if err != nil {
				t.Fatalf("io.Copy(w, io.TeeReader(f, h1)) error %s", err)
			}

			checksum1 := h1.Sum(nil)

			if err = w.Close(); err != nil {
				t.Fatalf("w.Cose() error %s", err)
			}
			t.Logf("compressed: %d, uncompressed: %d", buf.Len(), n1)

			r, err := NewReader2(buf, dictSize)
			if err != nil {
				t.Fatalf("NewReader2(buf) error %s", err)
			}

			h2 := sha256.New()
			n2, err := io.Copy(h2, r)
			if err != nil {
				t.Fatalf("io.Copy(h2, r) error %s", err)
			}
			if n2 != n1 {
				t.Fatalf("decompressed length %d; want %d", n2, n1)
			}

			checksum2 := h2.Sum(nil)

			if !bytes.Equal(checksum2, checksum1) {
				t.Fatalf("hash checksums differ")
			}
		})
	}
}
