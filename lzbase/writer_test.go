package lzbase

import (
	"bytes"
	"io"
	"testing"
)

const msg = `Das ist ein Test!
This is a test!
Это тест.
`

func TestWriterCycle(t *testing.T) {
	t.Skip("Writer.process doesn't work currently")
	var buf bytes.Buffer
	props, err := NewProperties(3, 0, 2)
	if err != nil {
		t.Fatalf("NewProperties: error %s", err)
	}
	const dictSize = 4096
	wd, err := NewWriterDict(dictSize, dictSize)
	if err != nil {
		t.Fatalf("NewWriterDict: error %v", err)
	}
	wstate := NewWriterState(props, wd)
	w := NewWriter(&buf, wstate, true)
	p := []byte(msg)
	n, err := w.Write(p)
	if err != nil {
		t.Fatalf("w.Write error %s", err)
	}
	if n != len(p) {
		t.Fatalf("w.Write wrote %d bytes; want %d", n, len(p))
	}
	if err = w.Close(); err != nil {
		t.Fatalf("w.Close error %s", err)
	}

	rd, err := NewReaderDict(dictSize, dictSize)
	if err != nil {
		t.Fatalf("NewReaderDict: error %v", err)
	}
	rstate := NewReaderState(props, rd)
	r, err := NewReader(&buf, rstate)
	if err != nil {
		t.Fatalf("NewReader error %v", err)
	}
	q := make([]byte, len(p))
	n, err = r.Read(q)
	if err != nil {
		t.Fatalf("r.Read error %v", err)
	}
	if n != len(p) {
		t.Fatalf("r.Read read %d bytes; want %d", n, len(p))
	}
	if !bytes.Equal(p, q) {
		t.Fatalf("r.Read returned %q; want %q", q, msg)
	}
	n, err = r.Read(q)
	if err != io.EOF {
		t.Fatalf("r.Read returned %v; want %v", err, io.EOF)
	}
	if n != 0 {
		t.Fatalf("r.Read returned %d bytes; want %d", n, 0)
	}
}
