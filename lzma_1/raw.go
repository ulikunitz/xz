package lzma_1

import (
	"io"

	"github.com/ulikunitz/lz"
)

const EOSLen uint64 = 1<<64 - 1

type RawReaderConfig struct {
	DictSize   int
	Properties Properties
	Len        uint64
}

type RawReader struct {
	dict  lz.Buffer
	state state
	rd    rangeDecoder
	cfg   RawReaderConfig
}

func NewRawReader(z io.Reader, cfg RawReaderConfig) (r *RawReader, err error) {
	panic("TODO")
}

type RawWriterConfig struct {
	Properties       Properties
	LZCfg            lz.Configurator
	MaxCompressedLen int
	MaxLen           int
	EOS              bool
}

type RawWriter struct {
	cfg RawWriterConfig
}

func NewRawWriter(z io.Writer, cfg RawWriterConfig) (w *RawWriter, err error) {
	panic("TODO")
}
