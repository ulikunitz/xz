// Copyright 2014-2019 Ulrich Kunitz. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package xzinternals

import (
	"bytes"
	"testing"

	"github.com/ulikunitz/xz/filter"
)

func TestHeader(t *testing.T) {
	h := Header{Flags: CRC32}
	data, err := h.MarshalBinary()
	if err != nil {
		t.Fatalf("MarshalBinary error %s", err)
	}
	var g Header
	if err = g.UnmarshalBinary(data); err != nil {
		t.Fatalf("UnmarshalBinary error %s", err)
	}
	if g != h {
		t.Fatalf("unmarshalled %#v; want %#v", g, h)
	}
}

func TestFooter(t *testing.T) {
	f := Footer{IndexSize: 64, Flags: CRC32}
	data, err := f.MarshalBinary()
	if err != nil {
		t.Fatalf("MarshalBinary error %s", err)
	}
	var g Footer
	if err = g.UnmarshalBinary(data); err != nil {
		t.Fatalf("UnmarshalBinary error %s", err)
	}
	if g != f {
		t.Fatalf("unmarshalled %#v; want %#v", g, f)
	}
}

func TestRecord(t *testing.T) {
	r := Record{1234567, 10000}
	p, err := r.MarshalBinary()
	if err != nil {
		t.Fatalf("MarshalBinary error %s", err)
	}
	n := len(p)
	buf := bytes.NewReader(p)
	g, m, err := readRecord(buf)
	if err != nil {
		t.Fatalf("readFrom error %s", err)
	}
	if m != n {
		t.Fatalf("read %d bytes; wrote %d", m, n)
	}
	if g.unpaddedSize != r.unpaddedSize {
		t.Fatalf("got unpaddedSize %d; want %d", g.unpaddedSize,
			r.unpaddedSize)
	}
	if g.uncompressedSize != r.uncompressedSize {
		t.Fatalf("got uncompressedSize %d; want %d", g.uncompressedSize,
			r.uncompressedSize)
	}
}

func TestIndex(t *testing.T) {
	records := []Record{{1234, 1}, {2345, 2}}

	var buf bytes.Buffer
	n, err := WriteIndex(&buf, records)
	if err != nil {
		t.Fatalf("writeIndex error %s", err)
	}
	if n != int64(buf.Len()) {
		t.Fatalf("writeIndex returned %d; want %d", n, buf.Len())
	}

	// indicator
	c, err := buf.ReadByte()
	if err != nil {
		t.Fatalf("buf.ReadByte error %s", err)
	}
	if c != 0 {
		t.Fatalf("indicator %d; want %d", c, 0)
	}

	g, m, err := readIndexBody(&buf)
	if err != nil {
		for i, r := range g {
			t.Logf("records[%d] %v", i, r)
		}
		t.Fatalf("readIndexBody error %s", err)
	}
	if m != n-1 {
		t.Fatalf("readIndexBody returned %d; want %d", m, n-1)
	}
	for i, rec := range records {
		if g[i] != rec {
			t.Errorf("records[%d] is %v; want %v", i, g[i], rec)
		}
	}
}

func TestBlockHeader(t *testing.T) {
	h := BlockHeader{
		CompressedSize:   1234,
		UncompressedSize: -1,
		Filters:          []filter.Filter{filter.NewLZMAFilter(4096)},
	}
	data, err := h.MarshalBinary()
	if err != nil {
		t.Fatalf("MarshalBinary error %s", err)
	}

	r := bytes.NewReader(data)
	g, n, err := readBlockHeader(r)
	if err != nil {
		t.Fatalf("readBlockHeader error %s", err)
	}
	if n != len(data) {
		t.Fatalf("readBlockHeader returns %d bytes; want %d", n,
			len(data))
	}
	if g.CompressedSize != h.CompressedSize {
		t.Errorf("got compressedSize %d; want %d",
			g.CompressedSize, h.CompressedSize)
	}
	if g.UncompressedSize != h.UncompressedSize {
		t.Errorf("got uncompressedSize %d; want %d",
			g.UncompressedSize, h.UncompressedSize)
	}
	if len(g.Filters) != len(h.Filters) {
		t.Errorf("got len(filters) %d; want %d",
			len(g.Filters), len(h.Filters))
	}
	glf := g.Filters[0].(*filter.LZMAFilter)
	hlf := h.Filters[0].(*filter.LZMAFilter)
	if glf.GetDictCap() != hlf.GetDictCap() {
		t.Errorf("got dictCap %d; want %d", glf.GetDictCap(), hlf.GetDictCap())
	}
}
