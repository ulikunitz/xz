package lzma

import "testing"

func TestOpBuffer(t *testing.T) {
	b := newOpBuffer(10)
	for i := 0; i < b.len(); i++ {
		if i&1 == 0 {
			if err := b.writeOp(match{0, i}); err != nil {
				t.Fatalf("writeOp %d error %s", i, err)
			}
		} else {
			if err := b.writeOp(lit{byte(i)}); err != nil {
				t.Fatalf("writeOp %d error %s", i, err)
			}
		}
	}
	if err := b.writeOp(match{0, 666}); err != ErrNoSpace {
		t.Fatalf("writeOp error %v; want %v", err, ErrNoSpace)
	}
	for i := 0; i < b.len(); i++ {
		op, err := b.readOp()
		if err != nil {
			t.Fatalf("readOp %d error %s", i, err)
		}
		if i&1 == 0 {
			m, ok := op.(match)
			if !ok {
				t.Fatalf("%d got %v; want match", i, op)
			}
			if m.n != i {
				t.Fatalf("len %d; want %d", m.n, i)
			}
		} else {
			l, ok := op.(lit)
			if !ok {
				t.Fatalf("%d got %v; want lit", i, op)
			}
			if int(l.b) != i {
				t.Fatalf("b %d; want %d", l.b, i)
			}
		}
	}
	if _, err := b.readOp(); err == nil {
		t.Fatal("b.readOp no error; want an error")
	}

}
