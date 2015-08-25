// Copyright 2015 Ulrich Kunitz. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package newlzma

import (
	"bytes"
	"fmt"
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
	params := CodecParams{
		LC:      2,
		LP:      0,
		PB:      2,
		DictCap: minDictCap,
		BufCap:  minDictCap + 1024,
		Flags:   EOSMarker | NoUncompressedSize | NoCompressedSize,
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
	fmt.Println(">>> Decoding")
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
