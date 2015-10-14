package lzma2

import (
	"bytes"
	"errors"
	"fmt"
	"io"

	"github.com/ulikunitz/xz/lzma"
)

// Limits for the compressed and uncompressed size that can be stored in
// a single chunk.
const (
	CompressedLimit   = 1 << 16
	UncompressedLimit = 1 << (16 + 5)
)

// Parameters stores the parameters for the segment reader and writer.
type Parameters struct {
	// dictionary capacity
	DictCap int
	// buffer capacity
	BufCap int
	// literal context
	LC int
	// literal position bits
	LP int
	// position bits
	PB int
	// block size; for checkingt compression size
	BlockSize int
}

// Default provides the default parameters.
var Default = Parameters{
	LC:        3,
	LP:        0,
	PB:        2,
	DictCap:   8 * 1024 * 1024,
	BlockSize: 8 * 1024,
}

// swFlags provides a type for the segment writer flags.
type swFlags uint8

const (
	// LZMA reset
	swLRN swFlags = 1 << iota
	// dictionary reset required
	swD
)

// segementWriter supports the creation of a chunk sequence. Multiple
// segment writers can be used in parallel for different sections of a
// file.
type segmentWriter struct {
	w         io.Writer
	buf       bytes.Buffer
	params    lzma.CodecParams
	e         *lzma.Encoder
	blockSize int
	flags     swFlags
	props     lzma.Properties
}

// newSegmentWriter creates a segment writer.
func newSegmentWriter(w io.Writer, p Parameters) (sw *segmentWriter, err error) {
	if w == nil {
		panic("writer w is nil")
	}
	if p.BlockSize == 0 {
		p.BlockSize = Default.BlockSize
	}
	sw = &segmentWriter{
		w:         w,
		blockSize: p.BlockSize,
		flags:     swLRN | swD,
		params: lzma.CodecParams{
			DictCap:          p.DictCap,
			BufCap:           p.BufCap,
			CompressedSize:   CompressedLimit,
			UncompressedSize: UncompressedLimit,
			LC:               p.LC,
			LP:               p.LP,
			PB:               p.PB,
		},
	}
	sw.buf.Grow(CompressedLimit)
	sw.e, err = lzma.NewEncoder(&sw.buf, &sw.params)
	if err != nil {
		return nil, err
	}
	return sw, nil
}

// number of bytes that may be written by closing the stream writer. May
// be wrong check for it.
const margin = 5 + 11

// badCompressionRatio checks whether the number of uncompressed bytes
// by the encoder is less then the number of compressed bytes. Note that
// the Wash method of the encoder should be called before.
func (sw *segmentWriter) badCompressionRatio() bool {
	uncompressed := sw.e.Uncompressed()
	if uncompressed <= 0 {
		return false
	}
	compressed := sw.e.Compressed()
	return compressed >= uncompressed
}

func (sw *segmentWriter) writeUncompressedChunk() error {
	panic("TODO")
}

func (sw *segmentWriter) writeCompressedChunk() error {
	if err := sw.e.Close(); err != nil {
		return err
	}
	if sw.e.Uncompressed() == 0 {
		return nil
	}
	// debug check
	if sw.e.Compressed() != int64(sw.buf.Len()) {
		panic(fmt.Errorf(
			"writeCompressedChunk: sw.e.Compressed() %d; want %d",
			sw.e.Compressed(), sw.buf.Len()))
	}
	h := chunkHeader{
		ctype:        cL,
		uncompressed: uint32(sw.e.Uncompressed() - 1),
		compressed:   uint16(sw.e.Compressed() - 1),
	}
	if sw.flags&swLRN != 0 {
		sw.flags &^= swLRN
		h.ctype = cLRN
	}
	if sw.flags&swD != 0 {
		sw.flags &^= swD
		h.ctype = cLRND
	}
	if h.ctype != cL {
		h.props = sw.props
	}
	hdata, err := h.MarshalBinary()
	if err != nil {
		return err
	}
	if _, err = sw.w.Write(hdata); err != nil {
		return err
	}
	if _, err = sw.buf.WriteTo(sw.w); err != nil {
		return err
	}
	sw.buf.Reset()
	if err = sw.e.Reset(&sw.buf, &sw.params); err != nil {
		return err
	}
	return nil
}

var errClosed = errors.New("lzma2: writer closed")

func (sw *segmentWriter) Write(p []byte) (n int, err error) {
	if sw.w == nil {
		return 0, errClosed
	}
	n, err = sw.e.Write(p)
	if err == lzma.ErrCompressedLimit || err == lzma.ErrUncompressedLimit {
		err = sw.Flush()
	}
	return n, err
}

func (sw *segmentWriter) Flush() error {
	if sw.w == nil {
		return errClosed
	}
	return sw.writeCompressedChunk()
}

func (sw *segmentWriter) Close() error {
	if err := sw.Flush(); err != nil {
		return err
	}
	sw.w = nil
	return nil
}
