package lzma

import (
	"bytes"
	"crypto/sha256"
	"fmt"
	"io"
	"math/rand"
	"os"
	"strings"
	"testing"

	"github.com/ulikunitz/lz"
)

func TestChunkHeader(t *testing.T) {
	tests := []struct {
		hdr   chunkHeader
		wrong bool
	}{
		{hdr: chunkHeader{control: cEOS}},
		{hdr: chunkHeader{control: cU, size: 10}},
		{hdr: chunkHeader{control: cUD, size: 100000}, wrong: true},
	}

	for i, tc := range tests {
		t.Run(fmt.Sprint(i), func(t *testing.T) {
			q, err := tc.hdr.append(nil)
			if err != nil {
				if tc.wrong {
					return
				}
				t.Fatalf("hdr.append(p) error %s", err)
			}
			if tc.wrong {
				t.Fatal("hdr.append(p) successful")
			}
			g, err := parseChunkHeader(bytes.NewReader(q))
			if err != nil {
				t.Fatalf("parseChunkHeader(%02x): error %s",
					q, err)
			}
			if g != tc.hdr {
				t.Fatalf("parseChunkHeader(%02x): got %+v;"+
					" want %+v", q, g, tc.hdr)
			}
		})
	}
}

func TestChunkWriterReaderSimple(t *testing.T) {
	const s = "=====foofoobar==foobar===="

	var cw chunkWriter
	buf := new(bytes.Buffer)
	lzCfg := lz.DHPConfig{}
	parser, err := lzCfg.NewParser()
	if err != nil {
		t.Fatalf("lzcfg.NewParser() error %s", err)
	}
	if err = cw.init(buf, parser, []byte(s), Properties{3, 0, 2}); err != nil {
		t.Fatalf("cw.init() error %s", err)
	}
	if err = cw.Close(); err != nil {
		t.Fatalf("cw.Close() error %s", err)
	}

	var cr chunkReader
	dictSize := parser.BufferConfig().WindowSize
	if err = cr.init(buf, dictSize); err != nil {
		t.Fatalf("cr.init() error %s", err)
	}

	out := new(bytes.Buffer)
	if _, err = io.Copy(out, &cr); err != nil {
		t.Logf("out %q", out.String())
		t.Fatalf("io.Copy(out, cr) error %s", err)
	}
	g := out.String()

	if g != s {
		t.Fatalf("got %q; want %q", g, s)
	}
}

func TestChunkWriterReader(t *testing.T) {
	tests := []func() (io.Reader, error){
		func() (io.Reader, error) {
			return strings.NewReader("=====foofoobar==foobar===="),
				nil
		},
		func() (io.Reader, error) {
			f, err := os.Open("testdata/enwik7")
			if err != nil {
				return nil, err
			}
			return io.LimitReader(f, 300000), nil
		},
		func() (io.Reader, error) {
			return os.Open("testdata/enwik7")
		},
		func() (io.Reader, error) {
			return io.LimitReader(rand.New(rand.NewSource(99)),
				150000), nil
		},
		func() (io.Reader, error) {
			r1 := io.LimitReader(rand.New(rand.NewSource(99)),
				150000)
			f, err := os.Open("testdata/enwik7")
			if err != nil {
				return nil, err
			}
			r2 := io.LimitReader(f, 150000)
			return io.MultiReader(r1, r2), nil
		},
	}
	for i, tc := range tests {
		tc := tc
		t.Run(fmt.Sprintf("%d", i+1), func(t *testing.T) {
			r, err := tc()
			if err != nil {
				t.Fatalf("can't generate reader")
			}
			if c, ok := r.(io.Closer); ok {
				defer c.Close()
			}
			hIn := sha256.New()
			z := io.TeeReader(r, hIn)
			var cw chunkWriter
			lzCfg := lz.DHPConfig{}
			parser, err := lzCfg.NewParser()
			if err != nil {
				t.Fatalf("lzcfg.NewParser() error %s", err)
			}
			buf := new(bytes.Buffer)
			err = cw.init(buf, parser, nil, Properties{3, 0, 2})
			if err != nil {
				t.Fatalf("cw.init() error %s", err)
			}
			nIn, err := io.Copy(&cw, z)
			if err != nil {
				t.Fatalf("io.Copy error %s", err)
			}
			if err = cw.Close(); err != nil {
				t.Fatalf("cw.Close error %s", err)
			}
			hvIn := hIn.Sum(nil)
			t.Logf("uncompressed: %d bytes; compressed: %d bytes",
				nIn, buf.Len())

			var cr chunkReader
			dictSize := parser.BufferConfig().WindowSize
			t.Logf("dictSize: %d", dictSize)
			if err = cr.init(buf, dictSize); err != nil {
				t.Fatalf("cr.init() error %s", err)
			}

			hOut := sha256.New()
			nOut, err := io.Copy(hOut, &cr)
			if err != nil {
				t.Fatalf("io.Copy(hOut, cr) error %s", err)
			}
			if nOut != nIn {
				t.Fatalf("got %d bytes out; want %d bytes",
					nOut, nIn)
			}
			t.Logf("%d bytes", nOut)
			hvOut := hOut.Sum(nil)
			if !bytes.Equal(hvIn, hvOut) {
				t.Fatalf("got hash value %02x; want %02x",
					hvOut, hvIn)
			}
		})
	}
}

func TestChunkClose(t *testing.T) {
	const s = "=====foofoobar==foobar===="

	var cw chunkWriter
	buf := new(bytes.Buffer)
	lzCfg := lz.DHPConfig{}
	parser, err := lzCfg.NewParser()
	if err != nil {
		t.Fatalf("lzcfg.NewParser() error %s", err)
	}
	if err = cw.init(buf, parser, []byte(s), Properties{3, 0, 2}); err != nil {
		t.Fatalf("cw.init() error %s", err)
	}
	if err = cw.Close(); err != nil {
		t.Fatalf("cw.Close() error %s", err)
	}

	var cr chunkReader
	dictSize := parser.BufferConfig().WindowSize
	if err = cr.init(buf, dictSize); err != nil {
		t.Fatalf("cr.init() error %s", err)
	}

	out := new(bytes.Buffer)
	if _, err = io.Copy(out, &cr); err != nil {
		t.Logf("out %q", out.String())
		t.Fatalf("io.Copy(out, cr) error %s", err)
	}
	g := out.String()

	if g != s {
		t.Fatalf("got %q; want %q", g, s)
	}
}

func TestPeekChunkHeader(t *testing.T) {
	var hdr = chunkHeader{
		control: cUD,
		size:    256,
	}
	p, err := hdr.append(nil)
	if err != nil {
		t.Fatalf("hdr.append(nil) error %s", err)
	}
	hr := &hdrReader{r: bytes.NewReader(p)}
	h, n, err := peekChunkHeader(hr)
	if err != nil {
		t.Fatalf("peekChunkHeader error %s", err)
	}
	if n != 3 {
		t.Errorf("peekChunkHeader returned n=%d; want %d", n, 3)
	}
	if h != hdr {
		t.Errorf("peekChunkHeader h=%+v; want %+v", h, hdr)
	}
}
