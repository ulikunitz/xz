package lzbase

import "testing"

func TestNewWriterDict(t *testing.T) {
	wd, err := newWriterDict(10, 10)
	if err != nil {
		t.Fatalf("newWriterDict(10, 10): error %s", err)
	}
	bytes := []byte("abcdebcde")
	n, err := wd.Write(bytes)
	if err != nil {
		t.Fatalf("wd.Write(): error %s", err)
	}
	if n != len(bytes) {
		t.Fatalf("wd.Write() wrote %d bytes; want %d", n, len(bytes))
	}
	m, err := wd.AdvanceHead(n)
	if err != nil {
		t.Fatalf("wd.AdvanceHead(): error %s", err)
	}
	if m != n {
		t.Fatalf("wd.AdvanceHead() advanced %d bytes; want %d", m, n)
	}
	wantedOffsets := []int64{1, 5}
	offsets := wd.Offsets([]byte("bcde"))
	t.Logf("offsets: %v", offsets)
	if len(offsets) != len(wantedOffsets) {
		t.Fatalf("wd.Offsets() returned %d offsets; want %d",
			len(offsets), len(wantedOffsets))
	}
	for i, o := range wantedOffsets {
		if offsets[i] != o {
			t.Errorf("offsets[%d] is %d; want %d", i, offsets[i], o)
		}
	}
}
