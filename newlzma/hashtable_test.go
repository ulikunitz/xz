// Copyright 2015 Ulrich Kunitz. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package newlzma

import (
	"fmt"
	"testing"
)

func TestSlot(t *testing.T) {
	e := make([]uint32, slotEntries+10)
	for i := range e {
		e[i] = uint32(i * i)
	}
	var s slot
	for _, p := range e {
		s.PutEntry(p)
	}
	r := s.Entries()
	if len(r) != slotEntries {
		t.Fatalf("len(r) %d; want %d", len(r), slotEntries)
	}
	d := e[len(e)-slotEntries:]
	for i, p := range r {
		q := d[i]
		if p != q {
			t.Fatalf("r[%d]=%d unexpected; want %d", i, p, q)
		}
	}
}

func TestHashTable(t *testing.T) {
	ht, err := newHashTable(32, 2)
	if err != nil {
		t.Fatalf("newHashTable: error %s", err)
	}
	s := "abcabcdefghijklmn"
	n, err := ht.Write([]byte(s))
	if err != nil {
		t.Fatalf("ht.Write: error %s", err)
	}
	if n != len(s) {
		t.Fatalf("ht.Write returned %d; want %d", n, len(s))
	}
	tests := []struct {
		s string
		w string
	}{
		{"ab", "[17 14]"},
		{"bc", "[16 13]"},
		{"ca", "[15]"},
		{"xx", "[]"},
		{"gh", "[8]"},
		{"mn", "[2]"},
	}
	for _, c := range tests {
		distances, err := ht.Matches([]byte(c.s))
		if err != nil {
			t.Fatalf("Matches error %s", err)
		}
		d := fmt.Sprintf("%v", distances)
		t.Logf("{%q, %q},", c.s, d)
		/*
			if o != c.w {
				t.Errorf("%s: offsets %s; want %s", c.s, o, c.w)
			}
		*/
	}
}
