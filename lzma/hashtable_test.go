// Copyright 2015 Ulrich Kunitz. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package lzma

import (
	"fmt"
	"testing"
)

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
