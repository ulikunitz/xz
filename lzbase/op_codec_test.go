package lzbase

import (
	"bytes"
	"testing"
)

func TestOpCodec(t *testing.T) {
	const dictsize = 4096
	ops := []Operation{
		lit{'a'}, lit{'b'}, lit{'c'}, lit{'d'}, lit{'e'},
		match{5, 5}, match{10, 5}, match{15, 5},
		EOSOp,
	}
	buf := new(bytes.Buffer)
	props, err := NewProperties(3, 0, 2)
	if err != nil {
		t.Fatalf("NewProperties error %s", err)
	}
	wd, err := NewWriterDict(dictsize, 2*dictsize)
	if err != nil {
		t.Fatalf("NewWriterDict error %s", err)
	}
	wstate := NewState(props, wd)
	e, err := NewOpEncoder(buf, wstate)
	if err != nil {
		t.Fatalf("NewOpEncoder error %s", err)
	}
	n, err := e.WriteOps(ops)
	if err != nil {
		t.Fatalf("WriteOps error %s", err)
	}
	if err = e.Close(); err != nil {
		t.Fatalf("Close error %s", err)
	}
	if n != len(ops) {
		t.Fatalf("WriteOps wrote %d operations; want %d", n, len(ops))
	}
	rd, err := NewReaderDict(dictsize, dictsize)
	if err != nil {
		t.Fatalf("NewReaderDict error %s", err)
	}
	rstate := NewState(props, rd)
	d, err := NewOpDecoder(buf, rstate)
	if err != nil {
		t.Fatalf("NewOpDecoder error %s", err)
	}
	opbuf := make([]Operation, len(ops))
	n, err = d.ReadOps(opbuf)
	t.Logf("ReadOps returned %d, error %s", n, err)
	if err != nil && err != EOS {
		t.Fatalf("ReadOps error %s", err)
	}
	t.Logf("opbuf %#v", opbuf)
	if n != len(ops)-1 {
		t.Fatalf("ReadOps read %d operations; want %d", n, len(ops)-1)
	}
	if !equalOps(ops[:n], opbuf[:n]) {
		t.Fatalf("read operations differ from original operations")
	}
}
