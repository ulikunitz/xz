package lzma

import (
	"bytes"
	"fmt"
	"io"
	"testing"

	"github.com/ulikunitz/lz"
)

func TestChunkHeader(t *testing.T) {
	tests := []struct {
		hdr      chunkHeader
		wrong    bool
		parseEOF bool
	}{
		{hdr: chunkHeader{control: cEOS}, parseEOF: true},
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
			if tc.parseEOF {
				if err != io.EOF {
					t.Fatalf("parseChunkHeader(%02x)"+
						" expected EOF; got error %v",
						q, err)
				}
			} else if err != nil {
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
	var lzCfg = lz.Config{}
	lzCfg.ApplyDefaults()
	seq, err := lzCfg.NewSequencer()
	if err != nil {
		t.Fatalf("lzcfg.NewSequencer() error %s", err)
	}
	if err = cw.init(buf, seq, []byte(s), Properties{3, 0, 2}); err != nil {
		t.Fatalf("cw.init() error %s", err)
	}
	if err = cw.Close(); err != nil {
		t.Fatalf("cw.Close() error %s", err)
	}

	var cr chunkReader
	dictSize := seq.WindowPtr().WindowSize
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
