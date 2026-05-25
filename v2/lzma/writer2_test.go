// SPDX-FileCopyrightText: © 2014 Ulrich Kunitz
//
// SPDX-License-Identifier: BSD-3-Clause

package lzma

import (
	"bytes"
	"crypto/sha256"
	"fmt"
	"io"
	"math/rand"
	"os"
	"strings"
	"testing"

	"github.com/ulikunitz/xz/v2/internal/randtxt"
)

func TestWriter2Simple(t *testing.T) {
	const s = "=====foofoobar==foobar===="

	buf := new(bytes.Buffer)
	w, err := NewWriter2(buf)
	if err != nil {
		t.Fatalf("NewWriter2(buf) error %s", err)
	}
	windowSize := w.WindowSize()
	t.Logf("Size: %d", windowSize)

	if _, err = io.WriteString(w, s); err != nil {
		t.Fatalf("io.WriteString(w, %q) error %s", s, err)
	}

	if err = w.Close(); err != nil {
		t.Fatalf("w.Close() error %s", err)
	}

	t.Logf("buf.Len() %d; len(s) %d", buf.Len(), len(s))

	r, err := NewReader2(buf, windowSize)
	if err != nil {
		t.Fatalf("NewReader2(buf) error %s", err)
	}
	defer r.Close()

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
	type cfg struct {
		Workers    int
		BufferSize int
	}
	tests := []cfg{
		{Workers: 1},
		{BufferSize: 100000, Workers: 2},
		{BufferSize: 3e5},
	}

	for i, cfg := range tests {
		t.Run(fmt.Sprintf("%d", i+1), func(t *testing.T) {
			const file = "testdata/enwik7"
			f, err := os.Open(file)
			if err != nil {
				t.Fatalf("os.Open(%q) error %s", file, err)
			}
			defer f.Close()

			h1 := sha256.New()

			var options []Writer2Option

			if cfg.BufferSize > 0 {
				options = append(options,
					WithWindowSize(cfg.BufferSize/2),
					WithBufferSize(cfg.BufferSize))
			}
			if cfg.Workers > 0 {
				options = append(options, WithWorkers(cfg.Workers))
			}

			buf := new(bytes.Buffer)
			w, err := NewWriter2(buf, options...)
			if err != nil {
				t.Fatalf("NewWriter2 error %s", err)
			}
			defer w.Close()
			windowSize := w.WindowSize()
			t.Logf("dictSize: %d", windowSize)

			n1, err := io.Copy(w, io.TeeReader(f, h1))
			if err != nil {
				t.Fatalf("io.Copy(w, io.TeeReader(f, h1)) error %s", err)
			}

			checksum1 := h1.Sum(nil)

			if err = w.Close(); err != nil {
				t.Fatalf("w.Close() error %s", err)
			}
			t.Logf("compressed: %d, uncompressed: %d", buf.Len(), n1)

			r, err := NewReader2(buf, windowSize)
			if err != nil {
				t.Fatalf("NewReader2(buf) error %s", err)
			}
			defer r.Close()

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

func TestMTWriter(t *testing.T) {
	const txtlen = 1023
	buf := new(bytes.Buffer)
	io.CopyN(buf, randtxt.NewReader(rand.NewSource(41)), txtlen)
	txt := buf.String()

	buf.Reset()
	w, err := NewWriter2(buf, WithWorkers(8))
	if err != nil {
		t.Fatalf("NewWriter2 error %s", err)
	}
	defer w.Close()
	if _, err = io.WriteString(w, txt); err != nil {
		t.Fatalf("io.WriteString error %s", err)
	}
	if err = w.Close(); err != nil {
		t.Fatalf("w.Close() error %s", err)
	}
	windowSize := w.WindowSize()

	r, err := NewReader2(buf, windowSize)
	if err != nil {
		t.Fatalf("NewReader2 error %s", err)
	}
	defer r.Close()
	sb := new(strings.Builder)
	if _, err = io.Copy(sb, r); err != nil {
		t.Fatalf("io.Copy error %s", err)
	}
	if err = r.Close(); err != nil {
		t.Fatalf("r.Close error %s", err)
	}

	got := sb.String()
	if len(got) != len(txt) {
		t.Fatalf("got string with length %d; want %d",
			len(got), len(txt))
	}

	if got != txt {
		t.Fatalf("decompressed text differs from original text")
	}
}

func writer2Test(workers int) func(t *testing.T, data []byte) {
	return func(t *testing.T, data []byte) {
		buf := new(bytes.Buffer)
		w, err := NewWriter2(buf,
			WithWorkers(workers),
			WithWindowSize(4096))
		if err != nil {
			t.Fatalf("NewWriter2 error %s", err)
		}
		defer w.Close()
		windowSize := w.WindowSize()

		n, err := w.Write(data)
		if err != nil {
			t.Fatalf("w.Write(data) error %s", err)
		}
		if n != len(data) {
			t.Fatalf("w.Write(data) wrote %d bytes; want %d",
				n, len(data))
		}

		if err = w.Close(); err != nil {
			t.Fatalf("w.Close() error %s", err)
		}

		r, err := NewReader2(buf, windowSize, WithWorkers(workers))
		if err != nil {
			t.Fatalf("NewReader2 error %s", err)
		}
		defer r.Close()

		rbuf := new(bytes.Buffer)
		if _, err = io.Copy(rbuf, r); err != nil {
			t.Fatalf("io.Copy(rbuf, r) error %s", err)
		}

		if err = r.Close(); err != nil {
			t.Fatalf("r.Close() error %s", err)
		}

		g := rbuf.Bytes()
		if !bytes.Equal(g, data) {
			t.Fatalf("decompressed data differs from original data")
		}
	}
}

func FuzzWriter2ST(f *testing.F) {
	f.Add([]byte("a"))
	f.Add([]byte{})
	f.Add([]byte("abcabcabcabcabcabcabcabc"))
	f.Add([]byte("aaaaaaaaaaaaaaaaaaaaaaaaaa"))
	f.Fuzz(writer2Test(1))
}

func FuzzWriter2MT(f *testing.F) {
	f.Add([]byte("a"))
	f.Add([]byte{})
	f.Add([]byte("abcabcabcabcabcabcabcabc"))
	f.Add([]byte("aaaaaaaaaaaaaaaaaaaaaaaaaa"))
	f.Fuzz(writer2Test(2))
}
