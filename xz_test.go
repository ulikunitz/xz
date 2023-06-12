// SPDX-FileCopyrightText: © 2014 Ulrich Kunitz
//
// SPDX-License-Identifier: BSD-3-Clause

package xz_test

import (
	"archive/tar"
	"bufio"
	"bytes"
	"crypto/sha256"
	"io"
	"os"
	"testing"

	"github.com/ulikunitz/xz"
)

func TestPanic(t *testing.T) {
	data := []byte([]uint8{253, 55, 122, 88, 90, 0, 0, 0, 255, 18, 217, 65, 0, 189, 191, 239, 189, 191, 239, 48})
	t.Logf("%q", string(data))
	t.Logf("0x%02x", data)
	r, err := xz.NewReader(bytes.NewReader(data))
	if err != nil {
		t.Logf("xz.NewReader error %s", err)
		return
	}
	_, err = io.ReadAll(r)
	if err != nil {
		t.Logf("ioutil.ReadAll(r) error %s", err)
		return
	}
}

func FuzzXZ(f *testing.F) {
	f.Add(1, []byte(""))
	f.Add(3, []byte(""))
	const foobar = "====foofoobarfoobar tender==="
	f.Add(1, []byte(foobar))
	f.Add(10, []byte(foobar))
	f.Fuzz(func(t *testing.T, workers int, data []byte) {
		if !(0 <= workers && workers <= 32) {
			t.Skip()
		}
		wc := xz.WriterConfig{Workers: workers}
		wc.SetDefaults()
		var err error
		if err = wc.Verify(); err != nil {
			t.Skip()
		}
		h1 := sha256.New()
		var buf bytes.Buffer
		w, err := xz.NewWriterConfig(&buf, wc)
		if err != nil {
			t.Fatalf("NewWriterConfig(&buf, %+v) error %s", wc, err)
		}
		defer w.Close()
		mw := io.MultiWriter(w, h1)
		n, err := mw.Write(data)
		if err != nil {
			t.Fatalf("w.Write(data) error %s", err)
		}
		if n != len(data) {
			t.Fatalf("w.Write(data) got n=%d; want %d",
				n, len(data))
		}
		if err = w.Close(); err != nil {
			t.Fatalf("w.Close() error %s", err)
		}
		h2 := sha256.New()
		rc := xz.ReaderConfig{Workers: workers}
		rc.SetDefaults()
		if err = rc.Verify(); err != nil {
			t.Fatalf("rc.Verify() for %+v error %s", rc, err)
		}
		r, err := xz.NewReaderConfig(&buf, rc)
		if err != nil {
			t.Fatalf("xz.NewReaderConfig(&buf, %+v) error %s",
				rc, err)
		}
		defer r.Close()
		k, err := io.Copy(h2, r)
		if err != nil {
			t.Fatalf("io.Copy(h2, r) error %s", err)
		}
		if k != int64(len(data)) {
			t.Fatalf("io.Copy(h2, r) returned %d; want %d",
				k, len(data))
		}
		h1sum := h1.Sum(nil)
		h2sum := h2.Sum(nil)
		if !bytes.Equal(h1sum, h2sum) {
			t.Fatalf("hash sums differ")
		}
		if err = r.Close(); err != nil {
			t.Fatalf("r.Close() error %s", err)
		}
	})
}

// TestCombined addresses issue https://github.com/ulikunitz/xz/pull/54
func TestReadCombinedStreams(t *testing.T) {
	const file = "testdata/combined.tar.xz"
	f, err := os.Open(file)
	if err != nil {
		t.Fatalf("os.Open(%q) error %s", file, err)
	}
	defer f.Close()
	r, err := xz.NewReader(f)
	if err != nil {
		t.Fatalf("xz.NewReader error %s", err)
	}
	defer r.Close()

	br := bufio.NewReader(r)

	files := 0
	tr := tar.NewReader(br)
	for {
		h, err := tr.Next()
		if err != nil {
			if err == io.EOF {
				break
			}
			t.Fatalf("tr.Next error %s", err)
		}
		files++
		t.Logf("header: %s", h.Name)
	}

	// We have to jump over zero bytes. Option -i of tar.
loop:
	for {
		p, err := br.Peek(1024)
		if err != nil {
			t.Fatalf("br.Peek(%d) error %s", 1024, err)
		}
		for i, b := range p {
			if b != 0 {
				br.Discard(i)
				break loop
			}
		}
		br.Discard(1024)
	}

	tr = tar.NewReader(br)
	for {
		h, err := tr.Next()
		if err != nil {
			if err == io.EOF {
				break
			}
			t.Fatalf("tr.Next error %s", err)
		}
		files++
		t.Logf("header: %s", h.Name)
	}
	if files != 2 {
		t.Fatalf("read %d files; want %d", files, 2)
	}
}

func TestReadSingleStream(t *testing.T) {
	const file = "testdata/combined.tar.xz"
	f, err := os.Open(file)
	if err != nil {
		t.Fatalf("os.Open(%q) error %s", file, err)
	}
	defer f.Close()

	cfg := xz.ReaderConfig{SingleStream: true}
	r, err := xz.NewReaderConfig(f, cfg)
	if err != nil {
		t.Fatalf("xz.NewReaderConfig(f, %+v) error %s", cfg, err)
	}
	defer r.Close()

	files := 0
	tr := tar.NewReader(r)
	for {
		h, err := tr.Next()
		if err != nil {
			if err == io.EOF {
				break
			}
			t.Fatalf("tr.Next error %s", err)
		}
		files++
		t.Logf("header: %s", h.Name)
	}
	// we need to read trailing zeros
	n, err := io.Copy(io.Discard, r)
	t.Logf("%d bytes discarded", n)
	if err != nil {
		t.Fatalf("io.Copy(io.Discard, r) error %s", err)
	}

	r, err = xz.NewReaderConfig(f, cfg)
	if err != nil {
		t.Fatalf("xz.NewReaderConfig(f, %+v) error %s", cfg, err)
	}
	defer r.Close()

	tr = tar.NewReader(r)
	for {
		h, err := tr.Next()
		if err != nil {
			if err == io.EOF {
				break
			}
			t.Fatalf("tr.Next error %s", err)
		}
		files++
		t.Logf("header: %s", h.Name)
	}

	if files != 2 {
		t.Fatalf("read %d files; want %d", files, 2)
	}
}
