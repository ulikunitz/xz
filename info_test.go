package xz

import (
	"bufio"
	"io"
	"os"
	"testing"
)

type wrapReader struct {
	r io.Reader
}

func (r *wrapReader) Read(p []byte) (n int, err error) {
	return r.r.Read(p)
}

func wrap(r io.Reader) io.Reader {
	return &wrapReader{r}
}

func TestStat(t *testing.T) {
	tests := []struct {
		file string
		info Info
	}{
		{"combined.tar.xz",
			Info{
				Streams:      2,
				Blocks:       2,
				Compressed:   352,
				Uncompressed: 20 * 1024,
				Check:        CRC64,
			},
		},
		{"example.xz",
			Info{
				Streams:      1,
				Blocks:       1,
				Compressed:   96,
				Uncompressed: 39,
				Check:        CRC64,
			},
		},
		{"fox-check-none.xz",
			Info{
				Streams:      1,
				Blocks:       1,
				Compressed:   96,
				Uncompressed: 45,
				Check:        None,
			},
		},
		{"fox.xz",
			Info{
				Streams:      1,
				Blocks:       1,
				Compressed:   104,
				Uncompressed: 45,
				Check:        CRC64,
			},
		},
	}

	wrapTests := []struct {
		name string
		wrap func(io.Reader) io.Reader
	}{
		{"file", func(r io.Reader) io.Reader {
			return r
		}},
		{"bufio", func(r io.Reader) io.Reader {
			return bufio.NewReader(r)
		}},
		{"simple", func(r io.Reader) io.Reader {
			return wrap(r)
		}},
	}

	for _, w := range wrapTests {
		for _, tc := range tests {
			t.Run(w.name+"/"+tc.file, func(t *testing.T) {
				s := "testdata/" + tc.file
				f, err := os.Open(s)
				if err != nil {
					t.Fatalf("os.Open(%q) error %s", s, err)
				}
				defer f.Close()
				info, err := Stat(w.wrap(f), 0)
				if err != nil {
					t.Fatalf("Stat error %s", err)
				}
				if info != tc.info {
					t.Errorf("Stat(%q) = %v, want %v",
						tc.file, info, tc.info)
				}
			})
		}
	}
}
