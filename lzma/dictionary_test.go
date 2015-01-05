package lzma

import (
	"io"
	"math/rand"
	"os"
	"testing"
)

func fillRandomBuf(d []byte, r *rand.Rand) {
	for i := range d {
		d[i] = byte(r.Int31n(256))
	}
}

func TestReaderDict(t *testing.T) {
	DebugOn(os.Stderr)
	defer DebugOff()

	r := rand.New(rand.NewSource(15))
	buf := make([]byte, 30)
	d, err := newReaderDict(10, 20)
	if err != nil {
		t.Fatal("couldn't create reader dictionary.")
	}
	if cap(d.data) < 20 {
		t.Fatalf("cap(d.data) = %d; want at least %d", cap(d.data), 20)
	}
	t.Logf("d.data: [0:%d:%d]", len(d.data), cap(d.data))
	t.Logf("d %#v", d)
	buf = buf[:12]
	fillRandomBuf(buf, r)
	n, err := d.Write(buf)
	if err != nil {
		t.Fatalf("d.Write(buf): %s", err)
	}
	if n != len(buf) {
		t.Fatalf("d.Write(buf) returned %d; want %d", n, len(buf))
	}
	if d.total != int64(n) {
		t.Fatalf("d.total = %d; want %d", d.total, n)
	}
	if d.off != 0 {
		t.Fatalf("d.off = %d; want %d", d.off, 0)
	}
	buf = buf[:2]
	if n, err = d.Read(buf); err != nil {
		t.Fatalf("d.Read(buf): %s", err)
	}
	if n != 2 {
		t.Fatalf("d.Read(buf) = %d; want %d", n, 2)
	}
	t.Logf("d %#v", d)
	buf = buf[:10]
	fillRandomBuf(buf, r)
	if n, err = d.Write(buf); err != nil {
		t.Fatalf("d.Write(buf) #2: %s", err)
	}
	if n != len(buf) {
		t.Fatalf("d.Write(buf) #2 = %d; want %d", n, len(buf))
	}
	t.Logf("d %#v", d)
	buf = buf[:19]
	if n, err = d.Read(buf); err != nil {
		t.Logf("d.Len() %d", d.Len())
		t.Fatalf("d.Read(buf) #2: %s", err)
	}
	if n != 19 {
		t.Fatalf("d.Read(buf) #2 = %d; want %d", n, 19)
	}
}

func TestReaderDictCopyMatch(t *testing.T) {
	r := rand.New(rand.NewSource(15))
	buf := make([]byte, 30)
	p, err := newReaderDict(16, 10)
	if err != nil {
		t.Fatalf("readerDict.init: %s", err)
	}
	t.Logf("cap(p.data): %d", cap(p.data))
	buf = buf[:5]
	fillRandomBuf(buf, r)
	n, err := p.Write(buf)
	if err != nil {
		t.Fatalf("p.Write: %s\n", err)
	}
	if n != len(buf) {
		t.Fatalf("p.Write returned %d; want %d", n, len(buf))
	}
	t.Logf("p %#v", p)
	t.Log("copyMatch(2, 3)")
	if err = p.copyMatch(2, 3); err != nil {
		t.Fatal(err)
	}
	t.Logf("p %#v", p)
	t.Log("copyMatch(8, 8)")
	if err = p.copyMatch(8, 8); err != nil {
		t.Fatal(err)
	}
	t.Logf("p %#v", p)
	buf = buf[:30]
	if n, err = p.Read(buf); err != nil {
		t.Fatalf("Read: %s", err)
	}
	t.Logf("Read: %d", n)
	t.Log("copyMatch(2, 5)")
	if err = p.copyMatch(2, 5); err != nil {
		t.Fatal(err)
	}
	t.Logf("p %#v", p)
	if n, err = p.Read(buf); err != nil {
		t.Fatalf("Read: %s", err)
	}
	t.Logf("Read: %d", n)
	t.Log("copyMatch(2, 2)")
	if err = p.copyMatch(2, 2); err != nil {
		t.Fatal(err)
	}
	t.Logf("p %#v", p)
	if p.total != 23 {
		t.Fatalf("p.total %d; want %d", p.total, 23)
	}
}

func TestReaderDictReset(t *testing.T) {
	p, err := newReaderDict(10, 10)
	if err != nil {
		t.Fatalf("readerDict.init: %s", err)
	}
	t.Logf("cap(p.data): %d", cap(p.data))
	r := rand.New(rand.NewSource(15))
	buf := make([]byte, 5)
	fillRandomBuf(buf, r)
	n, err := p.Write(buf)
	if err != nil {
		t.Fatalf("p.Write: %s\n", err)
	}
	if n != len(buf) {
		t.Fatalf("p.Write returned %d; want %d", n, len(buf))
	}
	if p.total != 5 {
		t.Fatalf("p.total %d; want %d", p.total, 5)
	}
	p.reset()
	if p.total != 0 {
		t.Fatalf("p.total after reset %d; want %d", p.total, 0)
	}
	n = p.readable()
	if n != 0 {
		t.Fatalf("p.readable() after reset %d; want %d", n, 0)
	}
}

func TestReaderDictEOF(t *testing.T) {
	p, err := newReaderDict(10, 10)
	if err != nil {
		t.Fatalf("newDecoderDict: %s", err)
	}
	r := rand.New(rand.NewSource(15))
	buf := make([]byte, 5)
	fillRandomBuf(buf, r)
	n, err := p.Write(buf)
	if err != nil {
		t.Fatalf("p.Write: %s\n", err)
	}
	if n != len(buf) {
		t.Fatalf("p.Write: returned %d; want %d", n, len(buf))
	}
	p.closed = true
	n, err = p.Read(buf)
	if err != nil {
		t.Fatalf("p.Read: error %s not expected", err)
	}
	if n != len(buf) {
		t.Fatalf("p.Read: got %d bytes; want %d", n, len(buf))
	}
	n, err = p.Read(buf)
	if err != io.EOF {
		t.Fatalf("p.Read: got err %s; want %s", err, io.EOF)
	}
	if n != 0 {
		t.Fatalf("p.Read: returned %d bytes; want %d", n, 0)
	}
}
