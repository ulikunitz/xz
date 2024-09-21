// SPDX-FileCopyrightText: © 2014 Ulrich Kunitz
//
// SPDX-License-Identifier: BSD-3-Clause

package lzma

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"reflect"
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
	cfg := WriterConfig{WindowSize: 4096}
	cfg.SetDefaults()
	if err := cfg.Verify(); err != nil {
		t.Fatalf("DictSize set without lzCfg: %s", err)
	}

	cfg = WriterConfig{
		ParserConfig: &lz.DHPConfig{WindowSize: 4097},
		WindowSize:   4098,
	}
	cfg.SetDefaults()
	bc := cfg.ParserConfig.BufConfig()
	if bc.WindowSize != 4098 {
		t.Fatalf("bc.windowSize %d; want %d", bc.WindowSize, 4098)
	}
}

func TestWriterConfigJSON(t *testing.T) {
	var err error
	var cfg WriterConfig
	cfg.SetDefaults()
	if err = cfg.Verify(); err != nil {
		t.Fatalf("Verify error %s", err)
	}
	p, err := json.MarshalIndent(&cfg, "", "  ")
	if err != nil {
		t.Fatalf("json.Marshal error %s", err)
	}
	t.Logf("json:\n%s", p)
	var cfg1 WriterConfig
	if err = json.Unmarshal(p, &cfg1); err != nil {
		t.Fatalf("json.Unmarshal error %s", err)
	}
	if !reflect.DeepEqual(cfg, cfg1) {
		t.Fatalf("json.Unmarshal: got %+v; want %+v",
			cfg1, cfg)
	}
}

func TestWriterConfigAll(t *testing.T) {
	tests := []string{
		`{"Format": "LZMA"}`,
	}
	for i, cfg := range tests {
		t.Run(fmt.Sprintf("%d", i+1), func(t *testing.T) {
			var c WriterConfig
			err := json.Unmarshal([]byte(cfg), &c)
			if err != nil {
				t.Fatalf("json.Unmarshal(%q, &c) failed: %v",
					cfg, err)
			}
		})
	}
}
