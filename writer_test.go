// Copyright 2014-2021 Ulrich Kunitz. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package xz

import (
	"bytes"
	"encoding/hex"
	"io"
	"log"
	"math/rand"
	"os"
	"testing"

	"github.com/ulikunitz/xz/internal/randtxt"
)

func TestWriter(t *testing.T) {
	const text = "The quick brown fox jumps over the lazy dog."
	var buf bytes.Buffer
	w, err := NewWriter(&buf)
	if err != nil {
		t.Fatalf("NewWriter error %s", err)
	}
	n, err := io.WriteString(w, text)
	if err != nil {
		t.Fatalf("WriteString error %s", err)
	}
	if n != len(text) {
		t.Fatalf("Writestring wrote %d bytes; want %d", n, len(text))
	}
	if err = w.Close(); err != nil {
		t.Fatalf("w.Close error %s", err)
	}
	var out bytes.Buffer
	r, err := NewReader(&buf)
	if err != nil {
		t.Fatalf("NewReader error %s", err)
	}
	if _, err = io.Copy(&out, r); err != nil {
		t.Fatalf("io.Copy error %s", err)
	}
	s := out.String()
	if s != text {
		t.Fatalf("reader decompressed to %q; want %q", s, text)
	}
}

func TestIssue12(t *testing.T) {
	var buf bytes.Buffer
	w, err := NewWriter(&buf)
	if err != nil {
		t.Fatalf("NewWriter error %s", err)
	}
	if err = w.Close(); err != nil {
		t.Fatalf("w.Close error %s", err)
	}
	r, err := NewReader(&buf)
	if err != nil {
		t.Fatalf("NewReader error %s", err)
	}
	var out bytes.Buffer
	if _, err = io.Copy(&out, r); err != nil {
		t.Fatalf("io.Copy error %s", err)
	}
	s := out.String()
	if s != "" {
		t.Fatalf("reader decompressed to %q; want %q", s, "")
	}
}

func Example() {
	const text = "The quick brown fox jumps over the lazy dog."
	var buf bytes.Buffer

	// compress text
	w, err := NewWriter(&buf)
	if err != nil {
		log.Fatalf("NewWriter error %s", err)
	}
	if _, err := io.WriteString(w, text); err != nil {
		log.Fatalf("WriteString error %s", err)
	}
	if err := w.Close(); err != nil {
		log.Fatalf("w.Close error %s", err)
	}

	// decompress buffer and write result to stdout
	r, err := NewReader(&buf)
	if err != nil {
		log.Fatalf("NewReader error %s", err)
	}
	if _, err = io.Copy(os.Stdout, r); err != nil {
		log.Fatalf("io.Copy error %s", err)
	}

	// Output:
	// The quick brown fox jumps over the lazy dog.
}

func TestWriter2(t *testing.T) {
	const txtlen = 1023
	var buf bytes.Buffer
	io.CopyN(&buf, randtxt.NewReader(rand.NewSource(41)), txtlen)
	txt := buf.String()

	buf.Reset()
	w, err := NewWriter(&buf)
	if err != nil {
		t.Fatalf("NewWriter error %s", err)
	}
	n, err := io.WriteString(w, txt)
	if err != nil {
		t.Fatalf("WriteString error %s", err)
	}
	if n != len(txt) {
		t.Fatalf("WriteString wrote %d bytes; want %d", n, len(txt))
	}
	if err = w.Close(); err != nil {
		t.Fatalf("Close error %s", err)
	}
	t.Logf("buf.Len() %d", buf.Len())

	r, err := NewReader(&buf)
	if err != nil {
		t.Fatalf("NewReader error %s", err)
	}
	var out bytes.Buffer
	k, err := io.Copy(&out, r)
	if err != nil {
		t.Fatalf("Decompressing copy error %s after %d bytes", err, n)
	}
	if k != txtlen {
		t.Fatalf("Decompression data length %d; want %d", k, txtlen)
	}
	if txt != out.String() {
		t.Fatal("decompressed data differs from original")
	}
}

func TestWriterNoneCheck(t *testing.T) {
	const txtlen = 1023
	var buf bytes.Buffer
	io.CopyN(&buf, randtxt.NewReader(rand.NewSource(41)), txtlen)
	txt := buf.String()

	buf.Reset()
	w, err := WriterConfig{NoCheckSum: true}.newWriter(&buf)
	if err != nil {
		t.Fatalf("NewWriter error %s", err)
	}
	n, err := io.WriteString(w, txt)
	if err != nil {
		t.Fatalf("WriteString error %s", err)
	}
	if n != len(txt) {
		t.Fatalf("WriteString wrote %d bytes; want %d", n, len(txt))
	}
	if err = w.Close(); err != nil {
		t.Fatalf("Close error %s", err)
	}
	t.Logf("buf.Len() %d", buf.Len())
	r, err := NewReader(&buf)
	if err != nil {
		t.Fatalf("NewReader error %s", err)
	}
	var out bytes.Buffer
	k, err := io.Copy(&out, r)
	if err != nil {
		t.Fatalf("Decompressing copy error %s after %d bytes", err, n)
	}
	if k != txtlen {
		t.Fatalf("Decompression data length %d; want %d", k, txtlen)
	}
	if txt != out.String() {
		t.Fatal("decompressed data differs from original")
	}
}

func BenchmarkWriter(b *testing.B) {
	const testFile = "testdata/enwik7"
	data, err := os.ReadFile(testFile)
	if err != nil {
		b.Fatalf("os.ReadFile(%q) error %s", testFile, err)
	}
	buf := new(bytes.Buffer)
	b.SetBytes(int64(len(data)))
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		buf.Reset()
		w, err := NewWriter(buf)
		if err != nil {
			b.Fatalf("NewWriter(buf) error %s", err)
		}
		if _, err = w.Write(data); err != nil {
			b.Fatalf("w.Write(data) error %s", err)
		}
		if err = w.Close(); err != nil {
			b.Fatalf("w.Write(data)")
		}
	}
	b.ReportMetric(float64(buf.Len())/float64(len(data)), "rate")
}

func TestWriteEmptyFile(t *testing.T) {
	buf := new(bytes.Buffer)
	w, err := NewWriter(buf)
	if err != nil {
		t.Fatalf("NewWriter(buf) error %s", err)
	}
	defer w.Close()
	if err = w.Flush(); err != nil {
		t.Fatalf("w.Flush() error %s", err)
	}
	if err = w.Close(); err != nil {
		t.Fatalf("w.Close() error %s", err)
	}

	if buf.Len() != 32 {
		t.Fatalf("xz file has length %d; want %d", buf.Len(), 32)
	}
	t.Logf("\n%s", hex.Dump(buf.Bytes()))

	r, err := NewReader(buf)
	if err != nil {
		t.Fatalf("NewReader(buf) error %s", err)
	}
	defer r.Close()
	p, err := io.ReadAll(r)
	if err != nil {
		t.Fatalf("io.ReadAll(r) error %s", err)
	}
	if err = r.Close(); err != nil {
		t.Fatalf("r.Close() error %s", err)
	}
	if len(p) != 0 {
		t.Fatalf("got len(p) %d; want %d", len(p), 0)
	}
}

func TestWriterFlush(t *testing.T) {
	buf := new(bytes.Buffer)
	w, err := NewWriter(buf)
	if err != nil {
		t.Fatalf("NewWriter(buf) error %s", err)
	}
	defer w.Close()

	if _, err = io.WriteString(w, "1"); err != nil {
		t.Fatalf("io.WriteString(%q) error %s", "1", err)
	}
	if err = w.Flush(); err != nil {
		t.Fatalf("w.Flush() error %s", err)
	}
	if _, err = io.WriteString(w, "2"); err != nil {
		t.Fatalf("io.WriteString(%q) error %s", "1", err)
	}
	if err = w.Close(); err != nil {
		t.Fatalf("w.Close() error %s", err)
	}

	t.Logf("\n%s", hex.Dump(buf.Bytes()))

	r, err := NewReader(buf)
	if err != nil {
		t.Fatalf("NewReader(buf) error %s", err)
	}
	defer r.Close()
	p, err := io.ReadAll(r)
	if err != nil {
		t.Fatalf("io.ReadAll(r) error %s", err)
	}
	if err = r.Close(); err != nil {
		t.Fatalf("r.Close() error %s", err)
	}

	s := string(p)
	if s != "12" {
		t.Fatalf("got string %q; want %s", s, "12")
	}
}
