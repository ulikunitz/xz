package xz

import (
	"bytes"
	"io"
	"testing"
)

func TestPadding(t *testing.T) {
	s := "abcd"
	for i := 1; i <= len(s); i++ {
		var buf bytes.Buffer
		w := newPadWriter(&buf, 4)
		n, err := io.WriteString(w, s[:i])
		if err != nil {
			t.Fatalf("WriteString error %s", err)
		}
		if n != i {
			t.Fatalf("WriteString wroted %d bytes; want %d", n, i)
		}
		n, err = w.Pad()
		if err != nil {
			t.Fatalf("w.pad() returned %s", err)
		}
		if n != 4-i {
			t.Fatalf("pad returned %d; want %d", n, 4-i)
		}
		n = buf.Len()
		if n != 4 {
			t.Fatalf("buf.Len() returned %d; want %d", n, 4)
		}
		p := make([]byte, i)
		r := newPadReader(&buf, 4)
		if n, err = io.ReadFull(r, p); err != nil {
			t.Fatalf("Read returned %s", err)
		}
		q := string(p)
		if q != s[:i] {
			t.Fatalf("Read returned %s; want %s", q, s[:i])
		}
		if n, err = r.Pad(); err != nil {
			t.Fatalf("Pad returned %s", err)
		}
		if n != 4-i {
			t.Fatalf("Pad returned %d; want %d", n, 4-i)
		}
	}
}
