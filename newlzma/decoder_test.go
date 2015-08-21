// Copyright 2015 Ulrich Kunitz. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package newlzma

import (
	"io"
	"io/ioutil"
	"os"
	"strings"
	"testing"
)

func TestDecoder(t *testing.T) {
	filename := "fox.lzma"
	want := "The quick brown fox jumps over the lazy dog.\n"
	for i := 0; i < 3; i++ {
		f, err := os.Open(filename)
		if err != nil {
			t.Fatalf("os.Open(%q) error %s", filename, err)
		}
		p := make([]byte, 13)
		_, err = io.ReadFull(f, p)
		if err != nil {
			t.Fatalf("io.Readfull error %s", err)
		}
		const capacity = 0x800000
		params := CodecParams{DictCap: capacity, BufCap: capacity}
		props := Properties(p[0])
		params.LC = props.LC()
		params.LP = props.LP()
		params.PB = props.PB()
		params.Flags = NoUncompressedSize | NoCompressedSize
		if i > 0 {
			params.Flags &^= NoUncompressedSize
			params.UncompressedSize = int64(len(want))
		}
		if i == 2 {
			params.Flags &^= NoCompressedSize
			fi, err := f.Stat()
			if err != nil {
				t.Fatalf("f.Stat error %s", err)
			}
			params.CompressedSize = fi.Size() - 13
		}
		r, err := NewDecoder(f, params)
		if err != nil {
			t.Fatalf("NewReader error %s", err)
		}
		bytes, err := ioutil.ReadAll(r)
		if err != nil {
			t.Logf("compressed size %d; want %d",
				r.Compressed(), params.CompressedSize)
			t.Fatalf("[%d] ReadAll error %s", i, err)
		}
		if err = f.Close(); err != nil {
			t.Fatalf("Close error %s", err)
		}
		got := string(bytes)
		if got != want {
			t.Fatalf("read %q; but want %q", got, want)
		}
	}
}

func TestDecoderUncompressed(t *testing.T) {
	want := "The quick brown fox jumps over the lazy dog.\n"
	f := strings.NewReader(want)
	const capacity = 0x800000
	params := CodecParams{DictCap: capacity, BufCap: capacity}
	params.Flags = Uncompressed
	params.UncompressedSize = int64(len(want))
	r, err := NewDecoder(f, params)
	if err != nil {
		t.Fatalf("NewReader error %s", err)
	}
	bytes, err := ioutil.ReadAll(r)
	if err != nil {
		t.Fatalf("ReadAll error %s", err)
	}
	got := string(bytes)
	if got != want {
		t.Fatalf("read %q; but want %q", got, want)
	}
}
