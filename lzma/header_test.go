// Copyright 2014-2022 Ulrich Kunitz. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package lzma

import "testing"

func TestHeaderMarshalling(t *testing.T) {
	tests := []Header{
		{Properties: Properties{3, 0, 2}, DictSize: 8 * 1024 * 1024,
			Size: -1},
		{Properties: Properties{4, 3, 3}, DictSize: 4096,
			Size: 10},
	}
	for _, h := range tests {
		data, err := h.marshalBinary()
		if err != nil {
			t.Fatalf("marshalBinary error %s", err)
		}
		var g Header
		if err = g.unmarshalBinary(data); err != nil {
			t.Fatalf("unmarshalBinary error %s", err)
		}
		if h != g {
			t.Errorf("got header %#v; want %#v", g, h)
		}
	}
}

func TestValidHeader(t *testing.T) {
	tests := []Header{
		{Properties: Properties{3, 0, 2}, DictSize: 8 * 1024 * 1024,
			Size: -1},
		{Properties: Properties{4, 3, 3}, DictSize: 4096,
			Size: 10},
	}
	for _, h := range tests {
		data, err := h.marshalBinary()
		if err != nil {
			t.Fatalf("marshalBinary error %s", err)
		}
		if !ValidHeader(data) {
			t.Errorf("ValidHeader returns false for header %v;"+
				" want true", h)
		}
	}
	const a = "1234567890123"
	if ValidHeader([]byte(a)) {
		t.Errorf("ValidHeader returns true for %s; want false", a)
	}
}
