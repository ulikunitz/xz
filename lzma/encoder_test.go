// Copyright 2015 Ulrich Kunitz. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package lzma

/*
import (
	"bytes"
	"io"
	"io/ioutil"
	"testing"
)

var testString = `LZMA decoder test example
=========================
! LZMA ! Decoder ! TEST !
=========================
! TEST ! LZMA ! Decoder !
=========================
---- Test Line 1 --------
=========================
---- Test Line 2 --------
=========================
=== End of test file ====
=========================
`

func TestEncoderCycle(t *testing.T) {
	params := &CodecParams{
		LC:      2,
		LP:      0,
		PB:      2,
		DictCap: minDictCap,
		BufCap:  minDictCap + 1024,
		Flags:   CEOSMarker | CNoUncompressedSize | CNoCompressedSize,
	}
	var buf bytes.Buffer
	w, err := NewEncoder(&buf, params)
	if err != nil {
		t.Fatalf("NewEncoder: error %s", err)
	}
	orig := []byte(testString)
	n, err := w.Write(orig)
	if err != nil {
		t.Fatalf("w.Write error %s", err)
	}
	if n != len(orig) {
		t.Fatalf("w.Write returned %d; want %d", n, len(orig))
	}
	if err = w.Close(); err != nil {
		t.Fatalf("w.Close error %s", err)
	}
	t.Logf("buf.Len() %d len(orig) %d", buf.Len(), len(orig))
	if buf.Len() > len(orig) {
		t.Errorf("buf.Len()=%d bigger then len(orig)=%d", buf.Len(),
			len(orig))
	}
	params.BufCap = params.DictCap
	r, err := NewDecoder(&buf, params)
	if err != nil {
		t.Fatalf("NewDecoder error %s", err)
	}
	decoded, err := ioutil.ReadAll(r)
	if err != nil {
		t.Fatalf("ReadAll(lr) error %s", err)
	}
	t.Logf("%s", decoded)
	if len(orig) != len(decoded) {
		t.Fatalf("length decoded is %d; want %d", len(decoded),
			len(orig))
	}
	if !bytes.Equal(orig, decoded) {
		t.Fatalf("decoded file differs from original")
	}
}

func TestEncoderUncompressed(t *testing.T) {
	params := &CodecParams{
		LC:               2,
		LP:               0,
		PB:               2,
		DictCap:          minDictCap,
		BufCap:           minDictCap + 1024,
		UncompressedSize: 1<<16 - 1,
		Flags:            CUncompressed | CNoCompressedSize,
	}
	var buf bytes.Buffer
	w, err := NewEncoder(&buf, params)
	if err != nil {
		t.Fatalf("NewEncoder: error %s", err)
	}
	orig := []byte(testString)
	n, err := w.Write(orig)
	if err != nil {
		t.Fatalf("w.Write error %s", err)
	}
	if n != len(orig) {
		t.Fatalf("w.Write returned %d; want %d", n, len(orig))
	}
	if err = w.Close(); err != nil {
		t.Fatalf("w.Close error %s", err)
	}
	t.Logf("buf.Len() %d len(orig) %d", buf.Len(), len(orig))
	if buf.Len() > len(orig) {
		t.Errorf("buf.Len()=%d bigger then len(orig)=%d", buf.Len(),
			len(orig))
	}
	params.UncompressedSize = int64(len(testString))
	params.BufCap = params.DictCap
	r, err := NewDecoder(&buf, params)
	if err != nil {
		t.Fatalf("NewDecoder error %s", err)
	}
	decoded, err := ioutil.ReadAll(r)
	if err != nil {
		t.Fatalf("ReadAll(lr) error %s", err)
	}
	t.Logf("%s", decoded)
	if len(orig) != len(decoded) {
		t.Fatalf("length decoded is %d; want %d", len(decoded),
			len(orig))
	}
	if !bytes.Equal(orig, decoded) {
		t.Fatalf("decoded file differs from original")
	}
}

func TestEncoderCopyDict(t *testing.T) {
	params := &CodecParams{
		LC:      2,
		LP:      0,
		PB:      2,
		DictCap: minDictCap,
		BufCap:  minDictCap + 1024,
		Flags:   CEOSMarker | CNoUncompressedSize | CNoCompressedSize,
	}
	var buf bytes.Buffer
	w, err := NewEncoder(&buf, params)
	if err != nil {
		t.Fatalf("NewEncoder: error %s", err)
	}
	n, err := io.WriteString(w, testString)
	if err != nil {
		t.Fatalf("w.Write error %s", err)
	}
	if n != len(testString) {
		t.Fatalf("w.Write returned %d; want %d", n, len(testString))
	}
	var buf2 bytes.Buffer
	n, err = w.CopyDict(&buf2, 4)
	if err != nil {
		t.Fatalf("w.CopyDict(&buf2, 4) error %s", err)
	}
	if n != 4 {
		t.Fatalf("w.CopyDict(&buf2, 4) returned %d; want %d", n, 4)
	}
	s := buf2.String()
	want := testString[len(testString)-4:]
	if s != want {
		t.Fatalf("buf2 contains %q; want %q", s, want)
	}
}
*/
