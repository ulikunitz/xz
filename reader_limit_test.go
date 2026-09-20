// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package xz

import (
	"bytes"
	"encoding/binary"
	"hash/crc32"
	"io"
	"strings"
	"testing"
	"testing/iotest"
)

// Keep fixtures small even if the limit check regresses.
func TestReaderWithMaxDictCap(t *testing.T) {
	payload := []byte("a small payload split across blocks")
	var buf bytes.Buffer
	w, err := (WriterConfig{DictCap: 4096, BlockSize: 16}).NewWriter(&buf)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = w.Write(payload); err != nil {
		t.Fatal(err)
	}
	if err = w.Close(); err != nil {
		t.Fatal(err)
	}
	good := append([]byte(nil), buf.Bytes()...)
	mutate := func(offset int) []byte {
		data := append([]byte(nil), good...)
		n := (int(data[offset]) + 1) * 4
		header := data[offset : offset+n]
		if header[1] != 0 || header[2] != 0x21 || header[3] != 1 {
			t.Fatalf("unexpected header: %x", header)
		}
		header[4] = 2 // 8 KiB
		binary.LittleEndian.PutUint32(header[n-4:],
			crc32.ChecksumIEEE(header[:n-4]))
		return data
	}
	first := mutate(HeaderLen)
	later := mutate(HeaderLen + int((w.index[0].unpaddedSize+3)&^3))
	concat := func(parts ...[]byte) []byte {
		return bytes.Join(parts, nil)
	}
	for _, tt := range []struct {
		name   string
		data   []byte
		config ReaderConfig
		limit  int
		want   []byte
		refuse bool
	}{
		{"boundary", good, ReaderConfig{}, 4096, payload, false},
		{"non-encodable limit", good, ReaderConfig{}, 4097, payload, false},
		{"larger limit", first, ReaderConfig{}, 8192, payload, false},
		{"first block", first, ReaderConfig{}, 4096, nil, true},
		{"no rounding up", first, ReaderConfig{}, 8191, nil, true},
		{"later block", later, ReaderConfig{}, 4096, payload[:16], true},
		{"later stream", concat(good, first), ReaderConfig{},
			4096, payload, true},
		{"padded later stream", concat(good, []byte{0, 0, 0, 0}, first),
			ReaderConfig{}, 4096, payload, true},
		{"concatenated streams", concat(good, good), ReaderConfig{},
			4096, concat(payload, payload), false},
		{"configured capacity", good, ReaderConfig{8192, false},
			4096, nil, true},
		{"configured boundary", good, ReaderConfig{8192, false},
			8192, payload, false},
		{"unchanged struct shape", good, ReaderConfig{4096, false},
			4096, payload, false},
		{"single stream", good, ReaderConfig{SingleStream: true},
			4096, payload, false},
	} {
		t.Run(tt.name, func(t *testing.T) {
			// Exercise fragmented input without changing the limit's scope.
			input := iotest.OneByteReader(bytes.NewReader(tt.data))
			r, err := tt.config.NewReaderWithMaxDictCap(input, tt.limit)
			if err != nil {
				t.Fatal(err)
			}
			got, err := io.ReadAll(r)
			if !bytes.Equal(got, tt.want) {
				t.Fatalf("decode %q; want %q", got, tt.want)
			}
			if tt.refuse {
				if err == nil || !strings.Contains(err.Error(), "exceeds maximum") {
					t.Fatalf("expected limit refusal, got %v", err)
				}
				return
			}
			if err != nil {
				t.Fatalf("decode %q: %v", got, err)
			}
		})
	}
	// The existing constructor still accepts the larger encoded dictionary.
	r, err := NewReader(bytes.NewReader(first))
	if err != nil {
		t.Fatal(err)
	}
	got, err := io.ReadAll(r)
	if err != nil || !bytes.Equal(got, payload) {
		t.Fatalf("legacy decode %q: %v", got, err)
	}
	// A rejected block must not consume its LZMA2 payload. The dictionary
	// check lives before Reader2 construction, which allocates and then
	// reads the first chunk header.
	input := bytes.NewReader(first)
	r, err = (ReaderConfig{}).NewReaderWithMaxDictCap(input, 4096)
	if err != nil {
		t.Fatal(err)
	}
	_, err = io.ReadAll(r)
	wantErr := "xz: dictionary capacity 8192 exceeds maximum 4096"
	if err == nil || err.Error() != wantErr {
		t.Fatalf("Read error %v; want %q", err, wantErr)
	}
	blockHeaderLen := (int(first[HeaderLen]) + 1) * 4
	if got := len(first) - input.Len(); got != HeaderLen+blockHeaderLen {
		t.Fatalf("read %d bytes; want only stream and block headers", got)
	}
	// SingleStream still rejects trailing streams instead of decoding them.
	r, err = (ReaderConfig{SingleStream: true}).NewReaderWithMaxDictCap(
		bytes.NewReader(concat(good, first)), 4096)
	if err != nil {
		t.Fatal(err)
	}
	got, err = io.ReadAll(r)
	if !bytes.Equal(got, payload) || err != errUnexpectedData {
		t.Fatalf("single-stream decode %q: %v", got, err)
	}
	// ReaderConfig is public and mutable; the constructor's cap is not.
	r, err = (ReaderConfig{}).NewReaderWithMaxDictCap(
		bytes.NewReader(concat(good, good)), 4096)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = io.ReadFull(r, make([]byte, len(payload))); err != nil {
		t.Fatal(err)
	}
	r.ReaderConfig = ReaderConfig{8192, false}
	_, err = io.ReadAll(r)
	if err == nil || !strings.Contains(err.Error(), "exceeds maximum") {
		t.Fatalf("mutable configuration relaxed the captured maximum: %v", err)
	}
}

func TestReaderWithMaxDictCapInvalidLimit(t *testing.T) {
	limits := []int{-1, 0, 1, 4095, int(^uint(0) >> 1)}
	// Exercise the format bound separately from platform-int overflow.
	tooLarge := uint64(1) << 32
	if uint64(^uint(0)>>1) >= tooLarge {
		limits = append(limits, int(tooLarge))
	}
	for _, limit := range limits {
		// A nil input also proves invalid limits are rejected before I/O.
		_, err := (ReaderConfig{}).NewReaderWithMaxDictCap(nil, limit)
		if err == nil || !strings.Contains(err.Error(),
			"maximum dictionary capacity") {
			t.Fatalf("invalid maximum %d: %v", limit, err)
		}
	}
}
