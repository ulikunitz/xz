package lzma

import (
	"bytes"
	"io"
	"strings"
	"testing"

	"github.com/ulikunitz/lz"
)

func TestWriterSimple(t *testing.T) {
	const s = "=====foofoobar==foobar===="

	buf := new(bytes.Buffer)
	w, err := NewWriter(buf)
	if err != nil {
		t.Fatalf("NewWriter(buf) error %s", err)
	}

	if _, err = io.WriteString(w, s); err != nil {
		t.Fatalf("io.WriteString(w, %q) error %s", s, err)
	}

	if err = w.Close(); err != nil {
		t.Fatalf("w.Close() error %s", err)
	}

	t.Logf("buf.Len() %d; len(s) %d", buf.Len(), len(s))

	r, err := NewReader(buf)
	if err != nil {
		t.Fatalf("NewReader(buf) error %s", err)
	}

	sb := new(strings.Builder)
	if _, err = io.Copy(sb, r); err != nil {
		t.Fatalf("io.Copy(sb, r) error %s", err)
	}

	g := sb.String()
	if g != s {
		t.Fatalf("got %q; want %q", g, s)
	}
}

func TestWriterConfigDictSize(t *testing.T) {
	cfg := WriterConfig{DictSize: 4096}
	cfg.ApplyDefaults()
	if err := cfg.Verify(); err != nil {
		t.Fatalf("DictSize set without lzCfg: %s", err)
	}

	params := lz.Params{WindowSize: 4097}
	lzCfg, err := lz.Config(params)
	if err != nil {
		t.Fatalf("lz.Config(%+v) error %s", params, err)
	}
	cfg = WriterConfig{
		LZ:       lzCfg,
		DictSize: 4098,
	}
	cfg.ApplyDefaults()
	sbCfg := cfg.LZ.BufferConfig()
	if sbCfg.WindowSize != 4098 {
		t.Fatalf("sbCfg.windowSize %d; want %d", sbCfg.WindowSize, 4098)
	}

}
