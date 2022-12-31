package tuning

import (
	"bytes"
	"io"
	"io/fs"

	"github.com/ulikunitz/xz"
)

type File struct {
	Name string
	Data []byte
}

func Files(corpus fs.FS) (files []File, err error) {
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
			files = append(files, File{Name: path, Data: data})
			return nil
		})
	return files, err
}

func Size(files []File) int64 {
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

func XZCompress(files []File, cfg xz.WriterConfig) (compressedSize int64, err error) {
	for _, f := range files {
		cw := &countWriter{}
		w, err := xz.NewWriterConfig(cw, cfg)
		if err != nil {
			return compressedSize, err
		}
		_, err = io.Copy(w, bytes.NewReader(f.Data))
		compressedSize += cw.n
		if err != nil {
			return compressedSize, err
		}
	}
	return compressedSize, nil
}
