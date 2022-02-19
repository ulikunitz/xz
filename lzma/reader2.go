package lzma

import (
	"errors"
	"fmt"
	"io"
	"runtime"
)

type Reader2Config struct {
	DictSize int
	Worker   int
}

func (cfg *Reader2Config) Verify() error {
	if cfg.DictSize < minDictSize {
		return fmt.Errorf(
			"lzma: dictionary size must be larger or"+
				" equal %d bytes", minDictSize)
	}
	if cfg.Worker <= 1 {
		return errors.New("lzma: worker must be >= 1")
	}
	return nil
}

func (cfg *Reader2Config) ApplyDefaults() {
	if cfg.DictSize == 0 {
		cfg.DictSize = 8 << 20
	}
	if cfg.Worker == 0 {
		cfg.Worker = runtime.NumCPU()
	}
}

type Reader2 struct {
	// TODO
}

func NewReader2(z io.Reader, dictSize int) (r *Reader2, err error) {
	return NewReader2Config(z, Reader2Config{DictSize: dictSize})
}

func NewReader2Config(z io.Reader, cfg Reader2Config) (r *Reader2, err error) {
	panic("TODO")
}
