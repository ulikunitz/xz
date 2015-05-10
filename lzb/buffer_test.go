package lzb

import (
	"bytes"
	"testing"
)

func TestInitBuffer(t *testing.T) {
	var b buffer
	const capacity = 30
	initBuffer(&b, capacity)
	if n := b.capacity(); n != capacity {
		t.Fatalf("capacity is %d; want %d", n, capacity)
	}
	if n := b.length(); n != 0 {
		t.Fatalf("length is %d; want %d", n, 0)
	}

}

func TestNewBuffer(t *testing.T) {
	const capacity = 30
	b := newBuffer(capacity)
	if n := b.capacity(); n != capacity {
		t.Fatalf("capacity is %d; want %d", n, capacity)
	}
	if n := b.length(); n != 0 {
		t.Fatalf("length is %d; want %d", n, 0)
	}
}

func TestBuffer_Write(t *testing.T) {
	var b buffer
	const capacity = 25
	initBuffer(&b, capacity)
	p := []byte("0123456789")
	n, err := b.Write(p)
	if err != nil {
		t.Fatalf("b.Write: unexpected error %s", err)
	}
	if n != len(p) {
		t.Fatalf("b.Write returned n=%d; want %d", n, len(p))
	}
	n = b.length()
	if n != len(p) {
		t.Fatalf("b.length is %d; want %d", n, len(p))
	}
	n, err = b.Write(p)
	if err != nil {
		t.Fatalf("b.Write: unexpected error %s", err)
	}
	if n != len(p) {
		t.Fatalf("b.Write returned n=%d; want %d", n, len(p))
	}
	if n = b.length(); n != 20 {
		t.Fatalf("data length %d; want %d", n, 20)
	}
	if !bytes.Equal(b.data[:10], p) {
		t.Fatalf("first 10 byte of data wrong")
	}
	if !bytes.Equal(b.data[10:20], p) {
		t.Fatalf("second batch of 10 bytes data wrong: %q", b.data[10:])
	}
	n, err = b.Write(p)
	if err != nil {
		t.Fatalf("b.Write: unexpected error %s", err)
	}
	if n != len(p) {
		t.Fatalf("b.Write returned n=%d; want %d", n, len(p))
	}
	if b.top != 30 {
		t.Fatalf("b.top is %d; want %d", b.top, 30)
	}
	if b.bottom != 5 {
		t.Fatalf("b.bottom is %d; want %d", b.bottom, 35)
	}
	t.Logf("b.data %q", b.data)
	if !bytes.Equal(b.data[:5], p[5:]) {
		t.Fatalf("b.Write overflow problem: b.data[:5] is %q; want %q",
			b.data[:5], p[5:])
	}
	q := make([]byte, 0, 30)
	for i := 0; i < 3; i++ {
		q = append(q, p...)
	}
	n, err = b.Write(q)
	if err != nil {
		t.Fatalf("b.Write: unexpected error %s", err)
	}
	if n != len(q) {
		t.Fatalf("b.Write returned n=%d; want %d", n, len(q))
	}
	if b.top != 60 {
		t.Fatalf("b.top is %d; want %d", b.top, 60)
	}
	if !bytes.Equal(b.data[10:], q[5:20]) {
		t.Fatalf("b.data[:10] is %q; want %q", b.data[:10], q[20:])
	}
	if !bytes.Equal(b.data[:10], q[20:]) {
		t.Fatalf("b.data[:10] is %q; want %q", b.data[:10], q[20:])
	}
	n, err = b.Write([]byte{})
	if err != nil {
		t.Fatalf("b.Write: error %s", err)
	}
	if n != 0 {
		t.Fatalf("b.Write([]byte{}) returned %d; want %d", n, 0)
	}
}

func TestBuffer_Write_limit(t *testing.T) {
	b := newBuffer(20)
	b.writeLimit = 9
	p := []byte("0123456789")
	n, err := b.Write(p)
	if err != errLimit {
		t.Fatalf("b.Write error %s; want %s", err, errLimit)
	}
	if n != 9 {
		t.Fatalf("n after b.Write %d; want %d", n, 9)
	}
	b.writeLimit += 10
	n, err = b.Write(p)
	if err != nil {
		t.Fatalf("b.Write error %s", err)
	}
	if n != 10 {
		t.Fatalf("n after b.Write %d; want %d", n, 10)
	}
}
