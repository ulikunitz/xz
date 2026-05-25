// SPDX-FileCopyrightText: © 2014 Ulrich Kunitz
//
// SPDX-License-Identifier: BSD-3-Clause

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
	type cfg struct {
		WindowSize int
		Workers    int
	}
	tests := []struct {
		wcfg cfg
		rcfg cfg
	}{
		{
			cfg{
				Workers:    3,
				WindowSize: 100000,
			},
			cfg{
				Workers:    3,
				WindowSize: 100000,
			},
		},
		{
			cfg{
				Workers:    3,
				WindowSize: 50000,
			},
			cfg{
				Workers:    3,
				WindowSize: 100000,
			},
		},
		{
			cfg{
				Workers:    3,
				WindowSize: 100000,
			},
			cfg{
				Workers:    3,
				WindowSize: 50000,
			},
		},
	}

	for i, tc := range tests {
		t.Run(fmt.Sprintf("%d", i+1), func(t *testing.T) {
			const file = "testdata/enwik7"
			f, err := os.Open(file)
			if err != nil {
				t.Fatalf("os.Open(%q) error %s", file, err)
			}
			defer f.Close()

			h1 := sha256.New()

			buf := new(bytes.Buffer)
			w, err := NewWriter2(buf,
				WithWindowSize(tc.wcfg.WindowSize),
				WithWorkers(tc.wcfg.Workers),
			)
			if err != nil {
				t.Fatalf("NewWriter2Options error %s", err)
			}
			defer w.Close()
			windowSize := w.WindowSize()

			n1, err := io.Copy(w, io.TeeReader(f, h1))
			if err != nil {
				t.Fatalf("io.Copy(w, io.TeeReader(f, h1)) error %s", err)
			}

			checksum1 := h1.Sum(nil)

			if err = w.Close(); err != nil {
				t.Fatalf("w.Close() error %s", err)
			}
			t.Logf("compressed: %d, uncompressed: %d", buf.Len(), n1)

			rcfg := tc.rcfg
			if rcfg.WindowSize < tc.wcfg.WindowSize {
				rcfg.WindowSize = windowSize
			}
			r, err := NewReader2(buf, rcfg.WindowSize,
				WithWorkers(rcfg.Workers),
			)
			if err != nil {
				t.Fatalf("NewReader2(buf, %+v) error %s",
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
