// SPDX-FileCopyrightText: © 2014 Ulrich Kunitz
//
// SPDX-License-Identifier: BSD-3-Clause

package lzma

import (
	"bytes"
	"io"
	"strings"
	"testing"
)

func TestWriterSimple(t *testing.T) {
	const s = "=====foofoobar==foobar===="

	buf := new(bytes.Buffer)
	w, err := NewWriter(buf, WithWindowSize(128))
	if err != nil {
		t.Fatalf("NewWriter(buf) error %s", err)
	}

	if _, err = io.WriteString(w, s); err != nil {
		t.Fatalf("io.WriteString(w, %q) error %s", s, err)
	}

	if err = w.Close(); err != nil {
		t.Fatalf("w.Close() error %s", err)
	}

	t.Logf("buf.Len() %d; len(s) %d", buf.Len(), len(s))

	r, err := NewReader(buf)
	if err != nil {
		t.Fatalf("NewReader(buf) error %s", err)
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

func FuzzWriter(f *testing.F) {
	f.Add([]byte("a"))
	f.Add([]byte{})
	f.Add([]byte("abcabcabcabcabcabcabcabc"))
	f.Add([]byte("aaaaaaaaaaaaaaaaaaaaaaaaaa"))
	f.Fuzz(func(t *testing.T, data []byte) {
		buf := new(bytes.Buffer)
		w, err := NewWriter(buf, WithWindowSize(128))
		if err != nil {
			t.Fatalf("NewWriter(buf) error %s", err)
		}
		defer w.Close()

		n, err := w.Write(data)
		if err != nil {
			t.Fatalf("w.Write(data) error %s", err)
		}
		if n != len(data) {
			t.Fatalf("w.Write(data) returned n=%d; want %d",
				n, len(data))
		}
		if err = w.Close(); err != nil {
			t.Fatalf("w.Close() error %s", err)
		}

		r, err := NewReader(buf)
		if err != nil {
			t.Fatalf("NewReader(buf) error %s", err)
		}

		rbuf := new(bytes.Buffer)
		c, err := io.Copy(rbuf, r)
		if err != nil {
			t.Fatalf("io.Copy(rbuf, r) error %s", err)
		}
		if c != int64(len(data)) {
			t.Fatalf("io.Copy(rbuf, r) returned n=%d; want %d",
				c, len(data))
		}

		gdata := rbuf.Bytes()
		if !bytes.Equal(gdata, data) {
			t.Fatalf("got %q; want %q", gdata, data)
		}
	})
}
