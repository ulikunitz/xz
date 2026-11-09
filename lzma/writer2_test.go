// Copyright 2014-2022 Ulrich Kunitz. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package lzma

import (
	"bytes"
	"errors"
	"io"
	"math/rand"
	"strings"
	"testing"

	"github.com/ulikunitz/xz/internal/randtxt"
)

func TestWriter2(t *testing.T) {
	var buf bytes.Buffer
	w, err := Writer2Config{DictCap: 4096}.NewWriter2(&buf)
	if err != nil {
		t.Fatalf("NewWriter error %s", err)
	}
	n, err := w.Write([]byte{'a'})
	if err != nil {
		t.Fatalf("w.Write([]byte{'a'}) error %s", err)
	}
	if n != 1 {
		t.Fatalf("w.Write([]byte{'a'}) returned %d; want %d", n, 1)
	}
	if err = w.Flush(); err != nil {
		t.Fatalf("w.Flush() error %s", err)
	}
	// check that double Flush doesn't write another chunk
	if err = w.Flush(); err != nil {
		t.Fatalf("w.Flush() error %s", err)
	}
	if err = w.Close(); err != nil {
		t.Fatalf("w.Close() error %s", err)
	}
	p := buf.Bytes()
	want := []byte{1, 0, 0, 'a', 0}
	if !bytes.Equal(p, want) {
		t.Fatalf("bytes written %#v; want %#v", p, want)
	}
}

func TestCycle1(t *testing.T) {
	var buf bytes.Buffer
	w, err := Writer2Config{DictCap: 4096}.NewWriter2(&buf)
	if err != nil {
		t.Fatalf("NewWriter error %s", err)
	}
	n, err := w.Write([]byte{'a'})
	if err != nil {
		t.Fatalf("w.Write([]byte{'a'}) error %s", err)
	}
	if n != 1 {
		t.Fatalf("w.Write([]byte{'a'}) returned %d; want %d", n, 1)
	}
	if err = w.Close(); err != nil {
		t.Fatalf("w.Close() error %s", err)
	}
	r, err := Reader2Config{DictCap: 4096}.NewReader2(&buf)
	if err != nil {
		t.Fatalf("NewReader error %s", err)
	}
	p := make([]byte, 3)
	n, err = r.Read(p)
	t.Logf("n %d error %v", n, err)
}

func TestCycle2(t *testing.T) {
	buf := new(bytes.Buffer)
	w, err := Writer2Config{DictCap: 4096}.NewWriter2(buf)
	if err != nil {
		t.Fatalf("NewWriter error %s", err)
	}
	// const txtlen = 1024
	const txtlen = 2100000
	io.CopyN(buf, randtxt.NewReader(rand.NewSource(42)), txtlen)
	txt := buf.String()
	buf.Reset()
	n, err := io.Copy(w, strings.NewReader(txt))
	if err != nil {
		t.Fatalf("Compressing copy error %s", err)
	}
	if n != txtlen {
		t.Fatalf("Compressing data length %d; want %d", n, txtlen)
	}
	if err = w.Close(); err != nil {
		t.Fatalf("w.Close error %s", err)
	}
	t.Logf("buf.Len() %d", buf.Len())
	r, err := Reader2Config{DictCap: 4096}.NewReader2(buf)
	if err != nil {
		t.Fatalf("NewReader error %s", err)
	}
	out := new(bytes.Buffer)
	n, err = io.Copy(out, r)
	if err != nil {
		t.Fatalf("Decompressing copy error %s after %d bytes", err, n)
	}
	if n != txtlen {
		t.Fatalf("Decompression data length %d; want %d", n, txtlen)
	}
	if txt != out.String() {
		t.Fatal("decompressed data differs from original")
	}
}

// TestWriter2SmallDictIncompressible checks that incompressible data is
// written correctly with a dictionary smaller than the data. Such data ends
// up in uncompressed chunks, which are copied out of the dictionary buffer,
// so no chunk may hold more data than that buffer.
func TestWriter2SmallDictIncompressible(t *testing.T) {
	for _, size := range []int{16 << 10, 64 << 10, 200 << 10} {
		data := make([]byte, size)
		rand.New(rand.NewSource(int64(size))).Read(data)
		var buf bytes.Buffer
		w, err := Writer2Config{DictCap: 4096}.NewWriter2(&buf)
		if err != nil {
			t.Fatalf("NewWriter2 error %s", err)
		}
		if _, err = w.Write(data); err != nil {
			t.Fatalf("size %d: Write error %s", size, err)
		}
		if err = w.Close(); err != nil {
			t.Fatalf("size %d: Close error %s", size, err)
		}
		r, err := Reader2Config{DictCap: 4096}.NewReader2(&buf)
		if err != nil {
			t.Fatalf("size %d: NewReader2 error %s", size, err)
		}
		out, err := io.ReadAll(r)
		if err != nil {
			t.Fatalf("size %d: ReadAll error %s", size, err)
		}
		if !bytes.Equal(out, data) {
			t.Fatalf("size %d: decompressed %d bytes; want %d", size, len(out), len(data))
		}
	}
}

type failWriter struct{}

func (failWriter) Write(p []byte) (int, error) {
	return 0, errors.New("failWriter: write failed")
}

// TestWriter2CloseReportsFlushError checks that Close doesn't swallow an
// error from flushing the last chunk.
func TestWriter2CloseReportsFlushError(t *testing.T) {
	w, err := Writer2Config{DictCap: 4096}.NewWriter2(failWriter{})
	if err != nil {
		t.Fatalf("NewWriter2 error %s", err)
	}
	if _, err = w.Write([]byte("hello")); err != nil {
		t.Fatalf("Write error %s", err)
	}
	if err = w.Close(); err == nil {
		t.Fatalf("Close returned nil although the final flush failed")
	}
}
