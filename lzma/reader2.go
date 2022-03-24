package lzma

import (
	"fmt"
	"io"
)

// Reader2Config provides the dictionary size parameter for a LZMA2 reader.
type Reader2Config struct {
	DictSize int
}

// Verify checks the validity of dictionary size.
func (cfg *Reader2Config) Verify() error {
	if cfg.DictSize < minDictSize {
		return fmt.Errorf(
			"lzma: dictionary size must be larger or"+
				" equal %d bytes", minDictSize)
	}
	return nil
}

// ApplyDefaults sets a default value for the dictionary size.
func (cfg *Reader2Config) ApplyDefaults() {
	if cfg.DictSize == 0 {
		cfg.DictSize = 8 << 20
	}
}

// NewReader2 creates a LZMA2 reader.
func NewReader2(z io.Reader, dictSize int) (r io.Reader, err error) {
	return NewReader2Config(z, Reader2Config{DictSize: dictSize})
}

// NewReader2Config genrates an LZMA2 reader using the configuration parameter
// attribute.
func NewReader2Config(z io.Reader, cfg Reader2Config) (r io.Reader, err error) {
	cfg.ApplyDefaults()
	if err = cfg.Verify(); err != nil {
		return nil, err
	}
	var cr chunkReader
	cr.init(z, cfg.DictSize)
	return &cr, nil
}
