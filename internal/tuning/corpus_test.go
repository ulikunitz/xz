package tuning

import (
	"bytes"
	"crypto/sha256"
	"io"
	"testing"

	"github.com/ulikunitz/xz"
	"github.com/ulikunitz/xz/lzma"
	"github.com/ulikunitz/zdata"
)

func TestSilesia(t *testing.T) {
	tests := []struct {
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
	}

	files, err := Files(zdata.Silesia)
	if err != nil {
		t.Fatalf("Files(zdata.Silesia) error %s", err)
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			for _, f := range files {
				s := sha256.Sum256(f.Data)
				hsum := s[:]

				buf := new(bytes.Buffer)
				w, err := xz.NewWriterConfig(buf, tc.cfg)
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
				r, err := xz.NewReaderConfig(buf, tc.rcfg)
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
					continue
				}
			}
		})
	}
}
