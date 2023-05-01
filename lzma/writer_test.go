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
	cfg.SetDefaults()
	if err := cfg.Verify(); err != nil {
		t.Fatalf("DictSize set without lzCfg: %s", err)
	}

	lzCfg := &lz.DHPConfig{WindowSize: 4097}
	cfg = WriterConfig{
		LZ:       lzCfg,
		DictSize: 4098,
	}
	cfg.SetDefaults()
	bc := cfg.LZ.BufConfig()
	if bc.WindowSize != 4098 {
		t.Fatalf("bc.windowSize %d; want %d", bc.WindowSize, 4098)
	}
}
