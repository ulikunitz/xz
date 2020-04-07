package xz

import (
	"bytes"
	"io"
	"io/ioutil"
	"os"
	"testing"
)

const foxSentenceConst = "The quick brown fox jumps over the lazy dog.\n"

func TestReaderAtBlocks(t *testing.T) {
	f, fileSize := testOpenFile(t, "testfiles/fox.blocks.xz")
	testFilePart(t, f, fileSize, foxSentenceConst, 0, len(foxSentenceConst))
}

func BenchmarkBlocks(b *testing.B) {
	f, fileSize := testOpenFile(b, "testfiles/fox.blocks.xz")

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		testFilePart(b, f, fileSize, foxSentenceConst, 0, len(foxSentenceConst))
	}
}

func TestReaderAtSimple(t *testing.T) {
	f, fileSize := testOpenFile(t, "testfiles/fox.xz")
	testFilePart(t, f, fileSize, foxSentenceConst, 0, 10)
}

func TestReaderAtMS(t *testing.T) {
	expect := foxSentenceConst + foxSentenceConst + foxSentenceConst + foxSentenceConst

	filePath := "testfiles/fox.blocks.xz"

	f, _ := testOpenFile(t, filePath)
	fData, err := ioutil.ReadAll(f)
	if err != nil {
		t.Fatalf("Error reading file %s", err)
	}
	msBytes := testMultiStreams(fData)
	msB := bytes.NewReader(msBytes)

	start := len(foxSentenceConst)
	testFilePart(t, msB, int64(len(msBytes)), expect, start, len(expect)-start)
}

func testOpenFile(t testing.TB, filePath string) (*os.File, int64) {
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

func testFilePart(t testing.TB, f io.ReaderAt, fileSize int64, expected string, start, size int) {
	conf := ReaderAtConfig{
		Len: fileSize,
	}
	r, err := conf.NewReaderAt(f)
	if err != nil {
		t.Fatalf("NewReader error %s", err)
	}

	decompressedBytes := make([]byte, size)
	n, err := r.ReadAt(decompressedBytes, int64(start))
	if err != nil {
		t.Fatalf("error while reading at: %v", err)
	}
	if n != len(decompressedBytes) {
		t.Fatalf("unexpectedly didn't read all")
	}

	subsetExpected := expected[start : start+size]
	if string(decompressedBytes) != subsetExpected {
		t.Fatalf("Unexpected decompression output. \"%s\" != \"%s\"",
			string(decompressedBytes), subsetExpected)
	}
}
