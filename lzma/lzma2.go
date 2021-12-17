package lzma

import (
	"io"
	"runtime"
)

// Reader2Config provides the configuration parameters for LZMA2 readers. The
// single Workers parameter provide the number of workes that should decompress
// in parallel.
type Reader2Config struct {
	Workers int
}

// NewReader2 returns a default reader for LZMA2 compressed files. It works
// single threaded.
func NewReader2(z io.Reader) (r io.Reader, err error) {
	return NewReader2Config(z, Reader2Config{Workers: 1})
}

// NewReader2Config generates readers for LZMA2 that may support parallel
// decompression if the LZMA2 stream contains dictionary resets.
func NewReader2Config(z io.Reader, cfg Reader2Config) (r io.Reader, err error) {
	panic("TODO")
}

// Writer2Config provides configuration parameters for LZMA2 writers. The
// MemoryBudget field must be bigger than the DictSize or both must be zero to
// be selected by the libary. Setting more than 1 workers will compress the data
// in parallel, which makes only sense if the data to compress is larger than
// the dictionary size, DictSize.
type Writer2Config struct {
	Properties
	// PropertiesInitialized indicates that LC, LP and PB should not be
	// changed.
	PropertiesInitialized bool
	DictSize              int
	MemoryBudget          int
	Effort                int
	Workers               int
}

// WriteFlusher is a Writer that can be closed and buffers flushed.
type WriteFlusher interface {
	io.WriteCloser
	Flush() error
}

// NewWriter2 creates a writer and support parallel compression.
func NewWriter2(z io.Writer) (w WriteFlusher, err error) {
	// TODO: test whether this is indeed the best setup.
	cfg := Writer2Config{
		Properties:            Properties{LC: 3, LP: 0, PB: 2},
		PropertiesInitialized: true,
		DictSize:              8 * mb,
		MemoryBudget:          10 * mb,
		Effort:                5,
		Workers:               runtime.NumCPU(),
	}
	return NewWriter2Config(z, cfg)
}

// NewWriter2Config creates a new compressing writer using the parameter in the
// cfg variable.
func NewWriter2Config(z io.Writer, cfg Writer2Config) (w WriteFlusher, err error) {
	panic("TODO")
}
