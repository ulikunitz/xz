package xz

import (
	"bytes"
	"crypto/sha256"
	"fmt"
	"io"
	"os"
	"testing"
)

func TestPreset(t *testing.T) {
	const file = "testdata/enwik7"
	for p := 1; p <= 9; p++ {
		t.Run(fmt.Sprintf("preset=%d", p), func(t *testing.T) {
			f, err := os.Open(file)
			if err != nil {
				t.Fatalf("os.Open(%q) error %s", file, err)
			}
			defer f.Close()
			cfg := Preset(p)
			h1 := sha256.New()
			var buf bytes.Buffer
			w, err := NewWriterConfig(&buf, cfg)
			if err != nil {
				t.Errorf("NewWriterConfig error %s", err)
				return
			}
			defer w.Close()
			n, err := io.Copy(io.MultiWriter(w, h1), f)
			if err != nil {
				t.Errorf("io.Copy error %s", err)
				return
			}
			if err = w.Close(); err != nil {
				t.Errorf("w.Close() error %s", err)
				return
			}

			c := buf.Len()
			ratio := float64(c) / float64(n)
			t.Logf("compression ratio: %5.2f%%", ratio*100)

			r, err := NewReader(&buf)
			if err != nil {
				t.Errorf("NewReader error %s", err)
				return
			}
			defer r.Close()
			h2 := sha256.New()
			_, err = io.Copy(h2, r)
			if err != nil {
				t.Errorf("io.Copy error %s", err)
				return
			}
			err = r.Close()
			if err != nil {
				t.Errorf("r.Close() error %s", err)
				return
			}
			h1sum := h1.Sum(nil)
			h2sum := h2.Sum(nil)
			if !bytes.Equal(h1sum, h2sum) {
				t.Errorf("checksums differ")
				return
			}
		})
	}

}
