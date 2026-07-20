package xz_test

import (
	"bytes"
	"io"
	"math/rand"
	"testing"

	"github.com/ulikunitz/xz"
	"github.com/ulikunitz/xz/internal/randtxt"
)

func BenchmarkDecompressionSpeed(b *testing.B) {
	// Подготовка данных (сжатие) происходит до старта таймера
	const size = 5 * 1024 * 1024
	r := io.LimitReader(randtxt.NewReader(rand.NewSource(42)), size)
	originalData, _ := io.ReadAll(r)

	var compressedBuf bytes.Buffer
	w, _ := xz.NewWriter(&compressedBuf)
	w.Write(originalData)
	w.Close()
	compressedData := compressedBuf.Bytes()

	b.SetBytes(int64(len(originalData)))
	b.ReportAllocs()
	b.ResetTimer() // Сбрасываем таймер перед основным циклом!

	for i := 0; i < b.N; i++ {
		inputReader := bytes.NewReader(compressedData)
		reader, _ := xz.NewReader(inputReader)
		io.Copy(io.Discard, reader) // Пишем в пустоту, чтобы мерить только LZMA
	}
}
