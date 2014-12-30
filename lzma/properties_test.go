package lzma

import (
	"bytes"
	"testing"
)

func TestVerify(t *testing.T) {
	err := verifyProperties(&defaultProperties)
	if err != nil {
		t.Errorf("verifyProperties(&defaultProperties) error %s", err)
	}
	err = verifyProperties(&Properties{})
	if err == nil {
		t.Fatal("verifyProperties(&Properties{}) no error")
	}
	t.Logf("verifyProperties(&Properties{}) error %s", err)
}

func TestWriteProperties(t *testing.T) {
	p := new(Properties)
	*p = defaultProperties
	buf := new(bytes.Buffer)
	var err error
	if err = writeProperties(buf, p); err != nil {
		t.Fatalf("writeProperties() error %s", err)
	}
	q, err := readProperties(buf)
	if err != nil {
		t.Fatalf("readProperties() error %s", err)
	}
	t.Logf("q %v", q)
	if err = verifyProperties(q); err != nil {
		t.Fatalf("verifyProperties(q) error %s", err)
	}
	if p.LC != q.LC {
		t.Errorf("q.LC %d; want %d", q.LC, p.LC)
	}
	if p.LP != q.LP {
		t.Errorf("q.LP %d; want %d", q.LP, p.LP)
	}
	if p.PB != q.PB {
		t.Errorf("q.PB %d; want %d", q.PB, p.PB)
	}
	if p.DictLen != q.DictLen {
		t.Errorf("q.DictLen %d; want %d", q.DictLen, p.DictLen)
	}
}
