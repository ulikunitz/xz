package lzma

import (
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io"
	"math/rand"
	"os"
	"reflect"
	"strings"
	"testing"

	"github.com/ulikunitz/lz"
	"github.com/ulikunitz/xz/internal/randtxt"
)

func TestWriter2Simple(t *testing.T) {
	const s = "=====foofoobar==foobar===="

	buf := new(bytes.Buffer)
	w, err := NewWriter2(buf)
	if err != nil {
		t.Fatalf("NewWriter2(buf) error %s", err)
	}
	dictSize := w.DictSize()
	t.Logf("dictSize: %d", dictSize)

	if _, err = io.WriteString(w, s); err != nil {
		t.Fatalf("io.WriteString(w, %q) error %s", s, err)
	}

	if err = w.Close(); err != nil {
		t.Fatalf("w.Close() error %s", err)
	}

	t.Logf("buf.Len() %d; len(s) %d", buf.Len(), len(s))

	r, err := NewReader2(buf, dictSize)
	if err != nil {
		t.Fatalf("NewReader2(buf) error %s", err)
	}
	defer r.Close()

	sb := new(strings.Builder)
	if _, err = io.Copy(sb, r); err != nil {
		t.Fatalf("io.Copy(sb, r) error %s", err)
	}

	g := sb.String()
	if g != s {
		t.Fatalf("got %q; want %q", g, s)
	}
}

func TestWriter2(t *testing.T) {
	tests := []Writer2Config{
		/*
			{Workers: 1},
			{WorkerBufferSize: 100000, Workers: 2},
		*/
		{WorkSize: 3e5},

		{},
	}

	for i, cfg := range tests {
		cfg := cfg
		t.Run(fmt.Sprintf("%d", i+1), func(t *testing.T) {
			const file = "testdata/enwik7"
			f, err := os.Open(file)
			if err != nil {
				t.Fatalf("os.Open(%q) error %s", file, err)
			}
			defer f.Close()

			h1 := sha256.New()

			buf := new(bytes.Buffer)
			w, err := NewWriter2Config(buf, cfg)
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
				t.Fatalf("w.Close() error %s", err)
			}
			t.Logf("compressed: %d, uncompressed: %d", buf.Len(), n1)

			r, err := NewReader2(buf, dictSize)
			if err != nil {
				t.Fatalf("NewReader2(buf) error %s", err)
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

func TestMTWriter(t *testing.T) {
	const txtlen = 1023
	buf := new(bytes.Buffer)
	io.CopyN(buf, randtxt.NewReader(rand.NewSource(41)), txtlen)
	txt := buf.String()

	buf.Reset()
	w, err := NewWriter2Config(buf, Writer2Config{Workers: 8})
	if err != nil {
		t.Fatalf("NewWriter2 error %s", err)
	}
	defer w.Close()
	if _, err = io.WriteString(w, txt); err != nil {
		t.Fatalf("io.WriteString error %s", err)
	}
	if err = w.Close(); err != nil {
		t.Fatalf("w.Close() error %s", err)
	}
	dictSize := w.DictSize()

	r, err := NewReader2(buf, dictSize)
	if err != nil {
		t.Fatalf("NewReader2 error %s", err)
	}
	defer r.Close()
	sb := new(strings.Builder)
	if _, err = io.Copy(sb, r); err != nil {
		t.Fatalf("io.Copy error %s", err)
	}
	if err = r.Close(); err != nil {
		t.Fatalf("r.Close error %s", err)
	}

	got := sb.String()
	if len(got) != len(txt) {
		t.Fatalf("got string with length %d; want %d",
			len(got), len(txt))
	}

	if got != txt {
		t.Fatalf("decompressed text differs from original text")
	}
}

func TestWriter2ConfigJSON(t *testing.T) {
	var err error
	var cfg Writer2Config
	cfg.SetDefaults()
	if err = cfg.Verify(); err != nil {
		t.Fatalf("Verify error %s", err)
	}
	p, err := json.MarshalIndent(&cfg, "", "  ")
	if err != nil {
		t.Fatalf("json.Marshal error %s", err)
	}
	t.Logf("json:\n%s", p)
	var cfg1 Writer2Config
	if err = json.Unmarshal(p, &cfg1); err != nil {
		t.Fatalf("json.Unmarshal error %s", err)
	}
	if !reflect.DeepEqual(cfg, cfg1) {
		t.Fatalf("json.Unmarshal: got %+v; want %+v",
			cfg1, cfg)
	}
}

func TestWriter2ConfigDictSize(t *testing.T) {
	cfg := Writer2Config{WindowSize: 4096}
	cfg.SetDefaults()
	if err := cfg.Verify(); err != nil {
		t.Fatalf("DictSize set without lzCfg: %s", err)
	}

	lzCfg := &lz.DHPConfig{WindowSize: 4097}
	cfg = Writer2Config{
		ParserConfig: lzCfg,
		WindowSize:   4098,
	}
	cfg.SetDefaults()
	bc := cfg.ParserConfig.BufConfig()
	if bc.WindowSize != 4098 {
		t.Fatalf("sbCfg.windowSize %d; want %d", bc.WindowSize, 4098)
	}
}
