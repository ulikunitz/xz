package xz

import (
	"bytes"
	"testing"
)

func TestHeader(t *testing.T) {
	flags := fCRC32
	var buf bytes.Buffer

	n, err := writeHeader(&buf, flags)
	if err != nil {
		t.Fatalf("writeHeader error %s", err)
	}
	if n != headerLen {
		t.Fatalf("writeHeader returned %d; want %d", n, headerLen)
	}

	g, m, err := readHeader(&buf)
	if err != nil {
		t.Fatalf("readHeader error %s", err)
	}
	if m != headerLen {
		t.Fatalf("readHeader returned %d; want %d", m, headerLen)
	}
	if g != flags {
		t.Fatalf("readHeader returned flags 0x%02x; want 0x%02x", g,
			flags)
	}
}

func TestFooter(t *testing.T) {
	flags := fCRC32
	indexSize := uint32(1234)
	var buf bytes.Buffer

	n, err := writeFooter(&buf, indexSize, flags)
	if err != nil {
		t.Fatalf("writeFooter error %s", err)
	}
	if n != footerLen {
		t.Fatalf("writeFooter returned %d; want %d", n, footerLen)
	}

	rIndexSize, rFlags, m, err := readFooter(&buf)
	if err != nil {
		t.Fatalf("readFooter error %s", err)
	}
	if m != footerLen {
		t.Fatalf("readFooter returned length %d; want %d", m, footerLen)
	}
	if rIndexSize != indexSize {
		t.Fatalf("readFooter returned index size %d; want %d",
			rIndexSize, indexSize)
	}
	if rFlags != flags {
		t.Fatalf("readFooter returned flags 0x%02x; want 0x%02x",
			rFlags, flags)
	}
}

func TestRecord(t *testing.T) {
	r := record{1234567, 10000}
	var buf bytes.Buffer
	n, err := r.writeTo(&buf)
	if err != nil {
		t.Fatalf("writeTo error %s", err)
	}
	var g record
	m, err := g.readFrom(&buf)
	if err != nil {
		t.Fatalf("readFrom error %s", err)
	}
	if m != n {
		t.Fatalf("read %d bytes; wrote %d", m, n)
	}
	if g.unpaddedSize != r.unpaddedSize {
		t.Fatalf("got unpaddedSize %d; want %d", g.unpaddedSize,
			r.unpaddedSize)
	}
	if g.uncompressedSize != r.uncompressedSize {
		t.Fatalf("got uncompressedSize %d; want %d", g.uncompressedSize,
			r.uncompressedSize)
	}
}

func TestIndex(t *testing.T) {
	records := []record{{1234, 1}, {2345, 2}}

	var buf bytes.Buffer
	n, err := writeIndex(&buf, records)
	if err != nil {
		t.Fatalf("writeIndex error %s", err)
	}
	if n != buf.Len() {
		t.Fatalf("writeIndex returned %d; want %d", n, buf.Len())
	}

	// indicator
	c, err := buf.ReadByte()
	if err != nil {
		t.Fatalf("buf.ReadByte error %s", err)
	}
	if c != 0 {
		t.Fatalf("indicator %d; want %d", c, 0)
	}

	g, m, err := readIndexBody(&buf)
	if err != nil {
		for i, r := range g {
			t.Logf("records[%d] %v", i, r)
		}
		t.Fatalf("readIndexBody error %s", err)
	}
	if m != n-1 {
		t.Fatalf("readIndexBody returned %d; want %d", m, n-1)
	}
	for i, rec := range records {
		if g[i] != rec {
			t.Errorf("records[%d] is %v; want %v", i, g[i], rec)
		}
	}
}
