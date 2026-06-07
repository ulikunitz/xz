package lzma

import (
	"os"
	"testing"
)

func TestStat(t *testing.T) {
	tests := []struct {
		filename     string
		uncompressed int64
		compressed   int64
	}{
		{"testdata/fox.lzma", 45, 67},
		{"testdata/examples/a.lzma", 327, 117},
		{"testdata/examples/a_eos_and_size.lzma", 327, 122},
		{"testdata/examples/a_eos.lzma", 327, 122},
		{"testdata/examples/a_lp1_lc2_pb1.lzma", 327, 117},
	}
	for _, tc := range tests {
		t.Run(tc.filename, func(t *testing.T) {
			f, err := os.Open(tc.filename)
			if err != nil {
				t.Fatal(err)
			}
			defer f.Close()
			info, err := Stat(f)
			if err != nil {
				t.Fatal(err)
			}
			t.Logf("info: %+v", info)
			if info.Compressed != tc.compressed {
				t.Fatalf("Compressed: got %d, want %d", info.Compressed, tc.compressed)
			}
			if info.Uncompressed != tc.uncompressed {
				t.Fatalf("Uncompressed: got %d, want %d", info.Uncompressed, tc.uncompressed)
			}
		})
	}
}
