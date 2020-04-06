package xz

import (
	"bytes"
	"io/ioutil"
	"os"
	"testing"
)

const expected = "The quick brown fox jumps over the lazy dog.\n"

func TestReaderAtBlocks(t *testing.T) {
	testFile(t, "testfiles/fox.blocks.xz", expected)
}

func TestReaderAtSimple(t *testing.T) {
	testFile(t, "testfiles/fox.xz", expected)
}

func TestReaderAtMS(t *testing.T) {
	expect := expected + expected + expected + expected
	filePath := "testfiles/fox.blocks.xz"

	f, _ := testOpenFile(t, filePath)
	fData, err := ioutil.ReadAll(f)
	if err != nil {
		t.Fatalf("Error reading file %s", err)
	}
	msBytes := testMultiStreams(fData)
	msB := bytes.NewReader(msBytes)

	conf := ReaderAtConfig{
		Len: int64(len(msBytes)),
	}
	r, err := conf.NewReaderAt(msB)
	if err != nil {
		t.Fatalf("NewReaderAt error %s", err)
	}

	reader := newRat(r, 0)
	decompressedBytes, err := ioutil.ReadAll(reader)
	if err != nil {
		t.Fatalf("io.Copy error %s", err)
	}

	if string(decompressedBytes) != expect {
		t.Fatalf("Unexpected decompression output for reader %+v. \"%s\" != \"%s\"", r, string(decompressedBytes), expect)
	}
}

func testOpenFile(t *testing.T, filePath string) (*os.File, int64) {
	xz, err := os.Open(filePath)
	if err != nil {
		t.Fatalf("os.Open(%q) error %s", filePath, err)
	}

	info, err := os.Stat(filePath)
	if err != nil {
		t.Fatalf("os.Stat(%q) error %s", filePath, err)
	}

	return xz, info.Size()
}

func testFile(t *testing.T, filePath string, expected string) {
	for i := 0; i < len(expected); i++ {
		for n := 1; n+i < len(expected); n++ {
			testFilePart(t, filePath, expected, i, n)
		}
	}
}

func testFilePart(t *testing.T, filePath string, expected string, start, size int) {
	f, fileSize := testOpenFile(t, filePath)

	conf := ReaderAtConfig{
		Len: fileSize,
	}
	r, err := conf.NewReaderAt(f)
	if err != nil {
		t.Fatalf("NewReader error %s", err)
	}

	decompressedBytes := make([]byte, size)
	n, err := r.ReadAt(decompressedBytes, int64(start))
	if n != len(decompressedBytes) {
		t.Fatalf("unexpectedly didn't read all")
	}
	if err != nil {
		t.Fatalf("io.Copy error %s", err)
	}

	subsetExpected := expected[start : start+size]
	if string(decompressedBytes) != subsetExpected {
		t.Fatalf("Unexpected decompression output. \"%s\" != \"%s\"", string(decompressedBytes), subsetExpected)
	}
}
