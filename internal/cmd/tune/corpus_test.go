package main

import (
	"bytes"
	"crypto/sha256"
	"io"
	"testing"

	"github.com/ulikunitz/lz"
	"github.com/ulikunitz/xz"
	"github.com/ulikunitz/xz/lzma"
)

func TestSilesia(t *testing.T) {
	if testing.Short() {
		t.Skip("slow test")
	}
	configs := []struct {
		name string
		cfg  xz.WriterConfig
		rcfg xz.ReaderConfig
	}{
		{"single-threaded", xz.WriterConfig{
			Workers: 1,
			LZMA:    lzma.Writer2Config{Workers: 1},
		},
			xz.ReaderConfig{
				Workers: 1,
				LZMA:    lzma.Reader2Config{Workers: 1},
			},
		},
		{"bug1",
			xz.WriterConfig{
				Workers: 1,
				LZMA: lzma.Writer2Config{
					Workers:  1,
					DictSize: 32768,
					LZ: &lz.HSConfig{
						InputLen: 3,
						HashBits: 4,
					},
				},
			},
			xz.ReaderConfig{
				Workers: 1,
				LZMA:    lzma.Reader2Config{Workers: 1},
			},
		},
	}

	files := silesiaFiles()

	for _, c := range configs {
		c := c
		for _, f := range files {
			f := f
			t.Run(c.name+":"+f.Name, func(t *testing.T) {
				t.Parallel()
				s := sha256.Sum256(f.Data)
				hsum := s[:]

				buf := new(bytes.Buffer)
				w, err := xz.NewWriterConfig(buf, c.cfg)
				if err != nil {
					t.Fatalf("xz.NewWriterConfig error %s",
						err)
				}
				defer w.Close()
				_, err = io.Copy(w, bytes.NewReader(f.Data))
				if err != nil {
					t.Fatalf("%s: io.Copy compression error %s",
						f.Name, err)
				}
				if err = w.Close(); err != nil {
					t.Fatalf("%s: w.Close() error %s",
						f.Name, err)
				}

				h := sha256.New()
				r, err := xz.NewReaderConfig(buf, c.rcfg)
				if err != nil {
					t.Fatalf("%s: xz.NewReaderConfig error %s",
						f.Name, err)
				}
				defer r.Close()
				_, err = io.Copy(h, r)
				if err != nil {
					t.Fatalf("%s: io.Copy decompression error %s",
						f.Name, err)
				}
				if err = r.Close(); err != nil {
					t.Fatalf("%s: r.Close() error %s",
						f.Name, err)
				}
				gsum := h.Sum(nil)
				if !bytes.Equal(gsum, hsum) {
					t.Errorf("%s: got %x; want %x",
						f.Name, gsum, hsum)
					return
				}
			})
		}
	}
}

func BenchmarkRatio(b *testing.B) {
	configs := []struct {
		name string
		cfg  xz.WriterConfig
	}{
			{name: "default-single-threaded",
				cfg: xz.WriterConfig{
					Workers: 1,
					LZMA:    lzma.Writer2Config{Workers: 1},
				},
			},
		{name: "hs3-15-st",
			cfg: xz.WriterConfig{
				Workers: 1,
				LZMA: lzma.Writer2Config{
					Workers: 1,
					LZ: &lz.HSConfig{
						InputLen: 3, HashBits: 15},
				},
			},
		},
		{name: "dhs3-15-st",
			cfg: xz.WriterConfig{
				Workers: 1,
				LZMA: lzma.Writer2Config{
					Workers: 1,
					LZ: &lz.DHSConfig{
						InputLen1: 3, HashBits1: 15,
						InputLen2: 6, HashBits2: 16},
				},
			},
		},

		{name: "buhs3-20-20-st",
			cfg: xz.WriterConfig{
				Workers: 1,
				LZMA: lzma.Writer2Config{
					Workers: 1,
					LZ: &lz.BUHSConfig{
						InputLen:   3,
						HashBits:   20,
						BucketSize: 20,
					},
				},
			},
		},
		{name: "DHS-4:11-7:14/67108864",
			cfg: xz.WriterConfig{
				Workers: 1,
				LZMA: lzma.Writer2Config{
					Workers:  1,
					DictSize: 67108864,
					LZ: &lz.DHSConfig{
						InputLen1: 4,
						HashBits1: 11,
						InputLen2: 7,
						HashBits2: 14,
					},
				},
			},
		},
		{name: "BUHS-3-20-128/8<<20",
			cfg: xz.WriterConfig{
				Workers: 1,
				LZMA: lzma.Writer2Config{
					Workers:  1,
					DictSize: 67108864,
					LZ: &lz.BUHSConfig{
						InputLen:   3,
						HashBits:   20,
						BucketSize: 128,
					},
				},
			},
		},
	}

	for _, c := range configs {
		b.Run(c.name, writerBenchmark(c.cfg))
	}
}
