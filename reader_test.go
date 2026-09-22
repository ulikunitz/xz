// Copyright 2014-2022 Ulrich Kunitz. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package xz

import (
	"bytes"
	"io"
	"io/ioutil"
	"os"
	"testing"
)

func TestReaderSimple(t *testing.T) {
	const file = "fox.xz"
	xz, err := os.Open(file)
	if err != nil {
		t.Fatalf("os.Open(%q) error %s", file, err)
	}
	r, err := NewReader(xz)
	if err != nil {
		t.Fatalf("NewReader error %s", err)
	}
	var buf bytes.Buffer
	if _, err = io.Copy(&buf, r); err != nil {
		t.Fatalf("io.Copy error %s", err)
	}
}

func TestReaderSingleStream(t *testing.T) {
	data, err := ioutil.ReadFile("fox.xz")
	if err != nil {
		t.Fatalf("ReadFile error %s", err)
	}
	xz := bytes.NewReader(data)
	rc := ReaderConfig{SingleStream: true}
	r, err := rc.NewReader(xz)
	if err != nil {
		t.Fatalf("NewReader error %s", err)
	}
	var buf bytes.Buffer
	if _, err = io.Copy(&buf, r); err != nil {
		t.Fatalf("io.Copy error %s", err)
	}
	buf.Reset()
	data = append(data, 0)
	xz = bytes.NewReader(data)
	r, err = rc.NewReader(xz)
	if err != nil {
		t.Fatalf("NewReader error %s", err)
	}
	if _, err = io.Copy(&buf, r); err != errUnexpectedData {
		t.Fatalf("io.Copy returned %v; want %v", err, errUnexpectedData)
	}
}

func TestReaderMultipleStreams(t *testing.T) {
	data, err := ioutil.ReadFile("fox.xz")
	if err != nil {
		t.Fatalf("ReadFile error %s", err)
	}
	m := make([]byte, 0, 4*len(data)+4*4)
	m = append(m, data...)
	m = append(m, data...)
	m = append(m, 0, 0, 0, 0)
	m = append(m, data...)
	m = append(m, 0, 0, 0, 0)
	m = append(m, 0, 0, 0, 0)
	m = append(m, data...)
	m = append(m, 0, 0, 0, 0)
	xz := bytes.NewReader(m)
	r, err := NewReader(xz)
	if err != nil {
		t.Fatalf("NewReader error %s", err)
	}
	var buf bytes.Buffer
	if _, err = io.Copy(&buf, r); err != nil {
		t.Fatalf("io.Copy error %s", err)
	}
}

func TestReaderTruncatedStreams(t *testing.T) {
	for _, payload := range [][]byte{nil, []byte("small payload split into several blocks")} {
		var compressed bytes.Buffer
		w, err := (WriterConfig{DictCap: 4096, BlockSize: 16}).NewWriter(&compressed)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := w.Write(payload); err != nil {
			t.Fatal(err)
		}
		if err := w.Close(); err != nil {
			t.Fatal(err)
		}
		good := compressed.Bytes()
		for _, later := range []bool{false, true} {
			for end := 0; end <= len(good); end++ {
				if later && end == 0 {
					// No second stream is an ordinary complete first stream.
					continue
				}
				data, want := good[:end], payload
				if later {
					data = append(append([]byte(nil), good...), data...)
					want = bytes.Repeat(payload, 2)
				}
				r, err := (ReaderConfig{DictCap: 4096}).NewReader(bytes.NewReader(data))
				var got []byte
				if err == nil {
					got, err = io.ReadAll(r)
				}
				if end == len(good) {
					if err != nil || !bytes.Equal(got, want) {
						t.Fatalf("payload=%d later=%v: complete stream returned %q, %v", len(payload), later, got, err)
					}
				} else if err == nil {
					t.Errorf("payload=%d later=%v: accepted truncated prefix %d/%d", len(payload), later, end, len(good))
				}
			}
		}
		if len(payload) == 0 {
			continue
		}
		// Both a missing block header and every partial header must report
		// truncation, rather than a successful stream end to io.ReadAll.
		blockHeaderLen := (int(good[HeaderLen]) + 1) * 4
		for end := HeaderLen; end < HeaderLen+blockHeaderLen; end++ {
			r, err := NewReader(bytes.NewReader(good[:end]))
			if err != nil {
				t.Fatal(err)
			}
			if _, err := io.ReadAll(r); err != io.ErrUnexpectedEOF {
				t.Errorf("block-header prefix %d/%d: got %v, want io.ErrUnexpectedEOF", end-HeaderLen, blockHeaderLen, err)
			}
		}
	}
}

func TestCheckNone(t *testing.T) {
	const file = "fox-check-none.xz"
	xz, err := os.Open(file)
	if err != nil {
		t.Fatalf("os.Open(%q) error %s", file, err)
	}
	r, err := NewReader(xz)
	if err != nil {
		t.Fatalf("NewReader error %s", err)
	}
	var buf bytes.Buffer
	if _, err = io.Copy(&buf, r); err != nil {
		t.Fatalf("io.Copy error %s", err)
	}
}

func BenchmarkReader(b *testing.B) {
	const testFile = "testdata/enwik7"
	data, err := os.ReadFile(testFile)
	if err != nil {
		b.Fatalf("os.ReadFile(%q) error %s", testFile, err)
	}
	buf := new(bytes.Buffer)
	uncompressedLen := int64(len(data))
	b.SetBytes(int64(uncompressedLen))
	b.ReportAllocs()
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
	data = make([]byte, buf.Len())
	copy(data, buf.Bytes())
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		buf.Reset()
		r, err := NewReader(bytes.NewReader(data))
		if err != nil {
			b.Fatalf("NewReader(data) error %s", err)
		}
		n, err := io.Copy(buf, r)
		if err != nil {
			b.Fatalf("io.Copy(buf, r) error %s", err)
		}
		if n != uncompressedLen {
			b.Fatalf("io.Copy got %d; want %d", n, uncompressedLen)
		}
	}
}
