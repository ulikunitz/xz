package xz

import (
	"bytes"
	"fmt"
	"math"
	"testing"
)

func sizeString(n int64) string {
	const (
		_MiB = 1 << 20
		_KiB = 1 << 10
	)
	if n == math.MaxUint32 {
		return "4096 MiB - 1 B"
	}
	var buf bytes.Buffer
	mib := n / _MiB
	n %= _MiB
	kib := n / _KiB
	n %= _KiB
	if mib != 0 {
		fmt.Fprintf(&buf, "%d MiB", mib)
	}
	if kib != 0 {
		if buf.Len() > 0 {
			fmt.Fprint(&buf, " ")
		}
		fmt.Fprintf(&buf, "%d KiB", kib)
	}
	if n != 0 {
		if buf.Len() > 0 {
			fmt.Fprint(&buf, " ")
		}
		fmt.Fprintf(&buf, "%d B", n)
	}
	return buf.String()
}

func TestLZMA2Flags(t *testing.T) {
	tests := []struct {
		f lzma2Flags
		s string
	}{
		{0, "4 KiB"},
		{1, "6 KiB"},
		{2, "8 KiB"},
		{3, "12 KiB"},
		{4, "16 KiB"},
		{5, "24 KiB"},
		{6, "32 KiB"},
		{35, "768 MiB"},
		{36, "1024 MiB"},
		{37, "1536 MiB"},
		{38, "2048 MiB"},
		{39, "3072 MiB"},
		{40, "4096 MiB - 1 B"},
	}
	for _, c := range tests {
		n, err := c.f.dictSize()
		if err != nil {
			t.Fatalf("f.dictSize: %s", err)
		}
		s := sizeString(n)
		if s != c.s {
			t.Errorf("size %q; want %q", s, c.s)
		}
	}
}
