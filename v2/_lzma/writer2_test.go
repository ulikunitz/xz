// SPDX-FileCopyrightText: © 2014 Ulrich Kunitz
//
// SPDX-License-Identifier: BSD-3-Clause

package lzma

import (
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io"
	"math/rand"
	"os"
	"reflect"
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
	tests := []Writer2Options{
		{Workers: 1},
		{BufferSize: 100000, Workers: 2},
		{BufferSize: 3e5},
		{},
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

			buf := new(bytes.Buffer)
			w, err := NewWriter2Options(buf, cfg)
			if err != nil {
				t.Fatalf("NewWriter2Config error %s", err)
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
	w, err := NewWriter2Options(buf, Writer2Options{Workers: 8})
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

func TestWriter2OptionsJSON(t *testing.T) {
	var err error
	var cfg Writer2Options
	cfg.setDefaults()
	if err = cfg.verify(); err != nil {
		t.Fatalf("Verify error %s", err)
	}
	p, err := json.MarshalIndent(&cfg, "", "  ")
	if err != nil {
		t.Fatalf("json.Marshal error %s", err)
	}
	t.Logf("json:\n%s", p)
	var cfg1 Writer2Options
	if err = json.Unmarshal(p, &cfg1); err != nil {
		t.Fatalf("json.Unmarshal error %s", err)
	}
	if !reflect.DeepEqual(cfg, cfg1) {
		if !reflect.DeepEqual(cfg.ParserOptions, cfg1.ParserOptions) {
			t.Fatalf("ParserOptions differ: got %+v; want %+v",
				cfg1.ParserOptions, cfg.ParserOptions)
		}
		t.Fatalf("json.Unmarshal: got %+v; want %+v",
			cfg1, cfg)
	}
}

func TestWriter2OptionsWindowSize(t *testing.T) {
	cfg := Writer2Options{WindowSize: 4096}
	cfg.setDefaults()
	if err := cfg.verify(); err != nil {
		t.Fatalf("WindowSize set without lzCfg: %s", err)
	}
}

/* TODO
func TestOptParser(t *testing.T) {
	const file = "../testdata/enwik7"
	const size = 500

	cfg := Writer2Options{
		ParserOptions: &OPConfig{
			InputLen: 4,
			HashBits: 24,
		},
	}

	var buf bytes.Buffer

	w, err := NewWriter2Config(&buf, cfg)
	if err != nil {
		t.Fatalf("NewWriter2Config error %s", err)
	}
	defer w.Close()
	dictSize := w.DictSize()

	h := sha256.New()
	mw := io.MultiWriter(w, h)

	f, err := os.Open(file)
	if err != nil {
		t.Fatalf("os.Open(%q) error %s", file, err)
	}
	defer f.Close()

	lf := io.LimitReader(f, size)

	if _, err = io.Copy(mw, lf); err != nil {
		t.Fatalf("io.Copy(mw, f) error %s", err)
	}

	if err = w.Close(); err != nil {
		t.Fatalf("w.Close() error %s", err)
	}

	sum1 := h.Sum(nil)

	t.Logf("compressed from %d to %d bytes", size, buf.Len())

	h.Reset()
	r, err := NewReader2(&buf, dictSize)
	if err != nil {
		t.Fatalf("NewReader2 error %s", err)
	}
	defer r.Close()

	if _, err = io.Copy(h, r); err != nil {
		t.Fatalf("io.Copy(h, r) error %s", err)
	}
	if err = r.Close(); err != nil {
		t.Fatalf("r.Close() error %s", err)
	}

	sum2 := h.Sum(nil)

	if !bytes.Equal(sum1, sum2) {
		t.Fatalf("hash sums differ")
	}
}
*/
