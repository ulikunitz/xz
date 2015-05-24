package lzb

import (
	"bytes"
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

func TestWriterCycle(t *testing.T) {
	params := Parameters{
		LC:         2,
		LP:         0,
		PB:         2,
		BufferSize: 4096,
		DictSize:   MinDictSize,
	}
	buf := new(bytes.Buffer)
	w, err := NewWriter(buf, params)
	if err != nil {
		t.Fatalf("NewWriter: error %s", err)
	}
	w.EOS = true
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
	r, err := NewReader(buf, params)
	if err != nil {
		t.Fatalf("NewReader error %s", err)
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
