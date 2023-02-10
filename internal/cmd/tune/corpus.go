package main

import (
	"bytes"
	"fmt"
	"io"
	"io/fs"
	"sync"
	"testing"

	"github.com/ulikunitz/xz"
	"github.com/ulikunitz/zdata"
)

type file struct {
	Name string
	Data []byte
}

func loadFiles(corpus fs.FS) (files []file, err error) {
	err = fs.WalkDir(corpus, ".",
		func(path string, entry fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if entry.IsDir() {
				return nil
			}
			data, err := fs.ReadFile(corpus, path)
			if err != nil {
				return err
			}
			files = append(files, file{Name: path, Data: data})
			return nil
		})
	return files, err
}

func totalSize(files []file) int64 {
	n := int64(0)
	for _, f := range files {
		n += int64(len(f.Data))
	}
	return n
}

type countWriter struct {
	n int64
}

func (w *countWriter) Write(p []byte) (n int, err error) {
	n = len(p)
	w.n += int64(n)
	return n, nil
}

func xzCompress(files []file, cfg xz.WriterConfig) (compressedSize int64, err error) {
	for _, f := range files {
		cw := &countWriter{}
		w, err := xz.NewWriterConfig(cw, cfg)
		if err != nil {
			return compressedSize, err
		}
		_, err = io.Copy(w, bytes.NewReader(f.Data))
		if err != nil {
			return compressedSize, err
		}
		if err = w.Close(); err != nil {
			return compressedSize, err
		}
		compressedSize += cw.n
		if err != nil {
			return compressedSize, err
		}
	}
	return compressedSize, nil
}

var (
	_silesiaFiles []file
	silesiaOnce   sync.Once
)

func silesiaFiles() []file {
	silesiaOnce.Do(func() {
		var err error
		_silesiaFiles, err = loadFiles(zdata.Silesia)
		if err != nil {
			panic(fmt.Errorf("silesiaFiles() error %w", err))
		}
	})
	return _silesiaFiles
}

func writerBenchmark(cfg xz.WriterConfig) func(b *testing.B) {
	return func(b *testing.B) {
		files := silesiaFiles()
		size := totalSize(silesiaFiles())
		b.SetBytes(size)
		var (
			err            error
			compressedSize int64
		)
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			compressedSize, err = xzCompress(files, cfg)
			if err != nil {
				b.Fatalf("XZCompress error %s", err)
			}
		}
		b.StopTimer()
		r := float64(compressedSize) / float64(size)
		b.ReportMetric(r, "c/u")
	}
}
