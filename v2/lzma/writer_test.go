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
	w, err := NewWriter(buf)
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

func FuzzLZMA(f *testing.F) {
	f.Add([]byte{})
	f.Add([]byte("foofoobar==foobar===="))
	f.Add([]byte("foofoobar==foobar====foofoobar==foobar===="))
	f.Fuzz(func(t *testing.T, data []byte) {
		buf := new(bytes.Buffer)
		w, err := NewWriter(buf)
		if err != nil {
			t.Fatalf("NewWriter(buf) error %s", err)
		}
		defer w.Close()

		if _, err = w.Write(data); err != nil {
			t.Fatalf("w.Write(data) error %s", err)
		}

		if err = w.Close(); err != nil {
			t.Fatalf("w.Close() error %s", err)
		}

		r, err := NewReader(buf)
		if err != nil {
			t.Fatalf("NewReader(buf) error %s", err)
		}

		buf2 := new(bytes.Buffer)
		if _, err = io.Copy(buf2, r); err != nil {
			t.Fatalf("io.Copy(buf2, r) error %s", err)
		}

		g := buf2.Bytes()
		if !bytes.Equal(g, data) {
			t.Fatalf("got %q; want %q", g, data)
		}
	})
}
