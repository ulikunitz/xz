package lzbase

import (
	"bytes"
	"testing"
)

func TestWriterCounter(t *testing.T) {
	buf := bytes.Buffer{}
	c := WriteCounter{W: &buf}
	c.Write([]byte("abc"))
	if c.N != 3 {
		t.Errorf("c.N is %d; want %d", c.N, 3)
	}
	c.Write([]byte("abcd"))
	if c.N != 7 {
		t.Errorf("c.N is %d; want %d", c.N, 7)
	}
}
