package lzma

import (
	"bytes"
	"crypto/sha256"
	"fmt"
	"io"
	"os"
	"testing"
)

func TestReader2(t *testing.T) {
	tests := []struct {
		wcfg Writer2Config
		rcfg Reader2Config
	}{
		{
			Writer2Config{
				Workers:          3,
				WorkerBufferSize: 100000,
			},
			Reader2Config{
				Workers:          3,
				WorkerBufferSize: 100000,
			},
		},
		{
			Writer2Config{
				Workers:          3,
				WorkerBufferSize: 50000,
			},
			Reader2Config{
				Workers:          3,
				WorkerBufferSize: 100000,
			},
		},
		{
			Writer2Config{
				Workers:          3,
				WorkerBufferSize: 100000,
			},
			Reader2Config{
				Workers:          3,
				WorkerBufferSize: 50000,
			},
		},

		{},
	}

	for i, tc := range tests {
		tc := tc
		t.Run(fmt.Sprintf("%d", i+1), func(t *testing.T) {
			const file = "testdata/enwik7"
			f, err := os.Open(file)
			if err != nil {
				t.Fatalf("os.Open(%q) error %s", file, err)
			}
			defer f.Close()

			h1 := sha256.New()

			buf := new(bytes.Buffer)
			w, err := NewWriter2Config(buf, tc.wcfg)
			if err != nil {
				t.Fatalf("NewWriter2Config error %s", err)
			}
			defer w.Close()
			dictSize := w.DictSize()
			t.Logf("dictSize: %d", dictSize)

			n1, err := io.Copy(w, io.TeeReader(f, h1))
			if err != nil {
				t.Fatalf("io.Copy(w, io.TeeReader(f, h1)) error %s", err)
			}

			checksum1 := h1.Sum(nil)

			if err = w.Close(); err != nil {
				t.Fatalf("w.Cose() error %s", err)
			}
			t.Logf("compressed: %d, uncompressed: %d", buf.Len(), n1)

			rcfg := tc.rcfg
			if rcfg.DictSize == 0 {
				rcfg.DictSize = dictSize
			}
			r, err := NewReader2Config(buf, rcfg)
			if err != nil {
				t.Fatalf("NewReader2Config(buf, %+v) error %s",
					rcfg, err)
			}
			defer r.Close()

			h2 := sha256.New()
			n2, err := io.Copy(h2, r)
			if err != nil {
				t.Fatalf("io.Copy(h2, r) error %s", err)
			}
			if n2 != n1 {
				t.Fatalf("decompressed length %d; want %d", n2, n1)
			}

			checksum2 := h2.Sum(nil)

			if !bytes.Equal(checksum2, checksum1) {
				t.Fatalf("hash checksums differ")
			}
		})
	}
}
