// Copyright 2014-2022 Ulrich Kunitz. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package lzma

import (
	"bytes"
	"io"
	"testing"
)

// stutterReader is a legal io.Reader that occasionally returns (0, nil).
type stutterReader struct {
	r     io.Reader
	stall bool
}

func (s *stutterReader) Read(p []byte) (int, error) {
	s.stall = !s.stall
	if s.stall {
		return 0, nil
	}
	return s.r.Read(p)
}

func TestBreaderZeroNilRead(t *testing.T) {
	data := []byte("hello, xz")
	br := ByteReader(&stutterReader{r: bytes.NewReader(data)})
	got := make([]byte, 0, len(data))
	for {
		c, err := br.ReadByte()
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatalf("ReadByte: %v", err)
		}
		got = append(got, c)
	}
	if !bytes.Equal(got, data) {
		t.Fatalf("got %q, want %q", got, data)
	}
}
