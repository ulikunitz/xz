// Copyright 2014-2022 Ulrich Kunitz. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package xz

import (
	"bytes"
	"testing"
)

func TestUvarint(t *testing.T) {
	tests := []uint64{0, 0x80, 0x100, 0xffffffff, 0x100000000, 1<<64 - 1}
	p := make([]byte, 10)
	for _, u := range tests {
		p = p[:10]
		n := putUvarint(p, u)
		if n < 1 {
			t.Fatalf("putUvarint returned %d", n)
		}
		r := bytes.NewReader(p[:n])
		x, m, err := readUvarint(r)
		if err != nil {
			t.Fatalf("readUvarint returned %s", err)
		}
		if m != n {
			t.Fatalf("readUvarint read %d bytes; want %d", m, n)
		}
		if x != u {
			t.Fatalf("readUvarint returned 0x%x; want 0x%x", x, u)
		}
	}
}

func TestUvarIntCVE_2020_16845(t *testing.T) {
	var a = []byte{0x81, 0x82, 0x83, 0x84, 0x85, 0x86, 0x87,
		0x88, 0x89, 0x8a, 0x8b}

	r := bytes.NewReader(a)
	_, _, err := readUvarint(r)
	if err != errOverflowU64 {
		t.Fatalf("readUvarint overflow not detected")
	}
}
