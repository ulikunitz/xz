package lzma

import (
	"math/rand"
	"testing"
)

func fillRandom(d []byte, r *rand.Rand) {
	for i := range d {
		d[i] = byte(r.Int31n(256))
	}
}

func TestDecoderDict(t *testing.T) {
	r := rand.New(rand.NewSource(15))
	buf := make([]byte, 30)
	d, err := newDecoderDict(20, 10)
	if err != nil {
		t.Fatal("Couldn't create decoder dictionary.")
	}
	if cap(d.data) != 20 {
		t.Fatalf("cap(d.data) = %d; want %d", cap(d.data), 20)
	}
	buf = buf[:12]
	fillRandom(buf, r)
	n, err := d.Write(buf)
	if err != nil {
		t.Fatalf("d.Write(buf): %s", err)
	}
	if n != len(buf) {
		t.Fatalf("d.Write(buf) returned %d; want %d", n, len(buf))
	}
	t.Logf("d %#v", d)
	if len(d.data) != n {
		t.Fatalf("len(d.data) = %d; want %d", len(d.data), n)
	}
	if d.c != n {
		t.Fatalf("d.c = %d; want %d", d.c, n)
	}
	if d.r != 0 {
		t.Fatalf("d.r = %d; want %d", d.r, 0)
	}
	buf = buf[:2]
	if n, err = d.Read(buf); err != nil {
		t.Fatalf("d.Read(buf): %s", err)
	}
	if n != 2 {
		t.Fatalf("d.Read(buf) = %d; want %d", n, 2)
	}
	t.Logf("d %#v", d)
	buf = buf[:9]
	fillRandom(buf, r)
	if n, err = d.Write(buf); err != nil {
		t.Fatalf("d.Write(buf) #2: %s", err)
	}
	if n != len(buf) {
		t.Fatalf("d.Write(buf) #2 = %d; want %d", n, len(buf))
	}
	if d.c != 1 {
		t.Fatalf("d.c = %d; want %d", d.c, 1)
	}
	if w := d.Writable(); w != 0 {
		t.Fatalf("d.Writable() = %d; want %d", w, 0)
	}
	t.Logf("d %#v", d)
	buf = buf[:19]
	if n, err = d.Read(buf); err != nil {
		t.Fatalf("d.Read(buf) #2: %s", err)
	}
}
