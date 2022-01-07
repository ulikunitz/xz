package lzma

import (
	"fmt"
	"io"
	"runtime"

	"github.com/ulikunitz/lz"
)

// NewReader2 returns a default reader for LZMA2 compressed files. It works
// single threaded.
func NewReader2(z io.Reader) (r io.Reader, err error) {
	panic("TODO")
}

// Writer2Config provides configuration parameters for LZMA2 writers. The
// MemoryBudget field must be bigger than the DictSize or both must be zero to
// be selected by the libary. Please note that the MemoryBudget applies per
// worker.
//
// If there are multiple workers configured, the written data will be split into
// dictSize segments and compressed in parallel.
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

type simpleByteWriter struct {
	data  []byte
	start int
}

func (bw *simpleByteWriter) Init(data []byte) {
	bw.data = data
	bw.start = len(data)
}

func (bw *simpleByteWriter) Len() int {
	return len(bw.data[bw.start:])
}

func (bw *simpleByteWriter) WriteByte(c byte) error {
	bw.data = append(bw.data, c)
	return nil
}

type segmentWriter struct {
	seq    lz.Sequencer
	enc    encoder
	blk    lz.Block
	seqIdx int
	litIdx int
}

func (w *segmentWriter) init(seq lz.Sequencer, props Properties) {
	*w = segmentWriter{
		seq: seq,
	}
	w.enc.init(nil, seq.WindowPtr(), props)
}

const (
	swDReset = 1 << iota
	swPReset
	swEOS
)

func (w *segmentWriter) compressChunk(in []byte, flags byte) (out []byte,
	rflags byte) {

	var (
		hdrLen int
		sel    chunkSelector
	)

	switch flags & (swDReset | swPReset) {
	case 0:
		hdrLen = 6
		sel = L2CSPD
	case swDReset:
		hdrLen = 6
		sel = L2CSP
	case swDReset | swPReset:
		hdrLen = 5
		sel = L2C
	default:
		panic(fmt.Errorf("invalid flags %b", flags))
	}

	var dummy [6]byte
	out = append(in, dummy[:hdrLen]...)

	var bw simpleByteWriter
	bw.Init(out)

	var backup encoder
	backup.clone(&w.enc)

	// TODO

	start := w.enc.pos
	endPos := w.enc.pos + maxChunkULen - maxMatchLen
	for bw.Len() < maxChunkLen-16 && w.enc.pos < endPos {
		switch {
		case len(w.blk.Sequences[w.seqIdx:]) > 0:
			// TODO: write next match
		case len(w.blk.Literals[w.litIdx:]) > 0:
			// TODO: write next literal
		default:
			// new block
		}
	}

	// TODO: remove
	_ = sel
	_ = start

	// TODO: return the right flags
	return bw.data, rflags
}

// compress2 compresses the given data into the slice out using seq and state
// into a standalone LZMA2 stream, requiring a directory reset and property
// update.
func (w *segmentWriter) compress2(in []byte, data []byte) (out []byte, err error) {
	out = in
	if err = w.seq.Reset(data); err != nil {
		return out, err
	}

	var flags byte
	for {
		out, flags = w.compressChunk(out, flags)
		if flags&swEOS != 0 {
			break
		}
	}

	return out, nil
}
