// SPDX-FileCopyrightText: © 2014 Ulrich Kunitz
//
// SPDX-License-Identifier: BSD-3-Clause

package lzma

import (
	"bufio"
	"errors"
	"fmt"
	"io"

	"github.com/ulikunitz/lz"
)

// default block size for parsing
const blockSize = 128 << 10

func setUpdateEncoder(p lz.Parser, updateEncoder func() *encoder) {
	/* TODO
	   op, ok := p.(*optParser)
	   if !ok {
	           return
	   }
	   op.updateEncoder = updateEncoder
	*/
}

// NewRawWriter writes only compress data stream. The argument eos controls
// whether an end of stream marker will be written.
func NewRawWriter(z io.Writer, parser lz.Parser, p Properties, eos bool) (w io.WriteCloser, err error) {
	if err = p.verify(); err != nil {
		return nil, err
	}

	wr := new(writer)
	if err = wr.init(z, parser, p, eos); err != nil {
		return nil, err
	}
	return wr, nil
}

// writer is a helper structure to implement writers. It provides the
// writeMatch and writeLiteral functions.
type writer struct {
	encoder
	parser lz.Parser
	blk    lz.Block
	eos    bool
	err    error
	bufw   *bufio.Writer
}

// init initializes a writer. eos tells the writer whether an end-of-stream
// marker should be written.
func (w *writer) init(z io.Writer, parser lz.Parser, p Properties, eos bool) error {
	var bufw *bufio.Writer
	bw, ok := z.(io.ByteWriter)
	if !ok {
		bufw = bufio.NewWriter(z)
		bw = bufw
	}
	*w = writer{
		parser:  parser,
		encoder: encoder{parser: parser},
		blk: lz.Block{
			Sequences: w.blk.Sequences[:0],
			Literals:  w.blk.Literals[:0],
		},

		bufw: bufw,
		eos:  eos,
	}

	w.state.init(p)
	w.re.init(bw)
	setUpdateEncoder(parser, func() *encoder {
		return &w.encoder
	})
	return nil
}

// Close closes the input stream.
func (w *writer) Close() error {
	if w.err != nil {
		return w.err
	}
	if w.err = w.clearBuffer(); w.err != nil {
		return w.err
	}
	if w.eos {
		if w.err = w.writeMatch(eosDist, minMatchLen); w.err != nil {
			return w.err
		}
	}
	if w.err = w.re.Close(); w.err != nil {
		return w.err
	}
	if w.bufw != nil {
		if w.err = w.bufw.Flush(); w.err != nil {
			return w.err
		}
	}
	w.err = errClosed
	return nil
}

// errClosed is returned if the object has already been closed.
var errClosed = errors.New("lzma: already closed")

// clearBuffer reads data from the buffer and encodes it.
func (w *writer) clearBuffer() error {
	for {
		_, err := w.parser.Parse(&w.blk, blockSize, 0)
		if err != nil {
			if err == lz.ErrEndOfBuffer {
				return nil
			}
			return err
		}

		var litIndex = 0
		for _, s := range w.blk.Sequences {
			i := litIndex
			litIndex += int(s.LitLen)
			for _, c := range w.blk.Literals[i:litIndex] {
				err = w.writeLiteral(c)
				if err != nil {
					return err
				}
			}

			// TODO: remove checks
			if s.Offset < minDistance {
				panic("s.Offset < minDistance")
			}
			if s.MatchLen < minMatchLen {
				panic("s.MatchLen < minMatchLen")
			}

			o, m := s.Offset-1, s.MatchLen
			for {
				var k uint32
				if m <= maxMatchLen {
					k = m
				} else if m >= maxMatchLen+minMatchLen {
					k = maxMatchLen
				} else {
					k = m - minMatchLen
				}
				if err = w.writeMatch(o, k); err != nil {
					return err
				}
				m -= k
				if m == 0 {
					break
				}
			}
		}
		for _, c := range w.blk.Literals[litIndex:] {
			if err = w.writeLiteral(c); err != nil {
				return err
			}
		}
	}
}

// Write write data into the buffer and encodes data if required.
func (w *writer) Write(p []byte) (n int, err error) {
	if w.err != nil {
		return 0, w.err
	}
	for {
		k, err := w.parser.Write(p[n:])
		n += k
		if err == nil {
			return n, nil
		}
		if err != lz.ErrFullBuffer {
			w.err = err
			return n, err
		}
		if err = w.clearBuffer(); err != nil {
			w.err = err
			return n, err
		}
	}
}

// limitWriter a simple writer ensuring a limit.
type limitWriter struct {
	n int64
	w writer
}

// Write writes data into the limited writer.
func (lw *limitWriter) Write(p []byte) (n int, err error) {
	if int64(len(p)) > lw.n {
		p = p[:lw.n]
		err = errors.New("lzma: file size reached")
	}
	var werr error
	n, werr = lw.w.Write(p)
	lw.n -= int64(n)
	if werr != nil {
		return n, werr
	}
	return n, err
}

// Close closes the writer and the underlying writer.
func (lw *limitWriter) Close() error {
	if lw.n > 0 {
		return errors.New("lzma: more data required")
	}
	return lw.w.Close()
}

// writerConfig defines the parameters for the LZMA Writer.
type writerConfig struct {
	// WindowSize defines the size of the sliding dictionary window for the
	// LZ parsing. If it is non-zero it overrides the parser configuration
	// of the lz package.
	WindowSize int

	// Properties of the LZMA algorithm.
	Properties Properties

	// FixedSize says that the stream has a fixed size known before
	// compression.
	FixedSize bool

	// Size gives the actual size if FixedSize is set.
	Size int64

	PathFinder string

	Mapper string

	MinMatchLen int
	MaxMatchLen int
}

type WriterOption interface {
	updateWriterConfig(*writerConfig) error
}

type windowSizeOption int

func (o windowSizeOption) updateWriterConfig(cfg *writerConfig) error {
	if o < 0 {
		return fmt.Errorf("lzma: window size must be non-negative")
	}
	cfg.WindowSize = int(o)
	return nil
}

func WithWindowSize(windowSize int) WriterOption {
	return windowSizeOption(windowSize)
}

type propertiesOption Properties

func (o propertiesOption) updateWriterConfig(cfg *writerConfig) error {
	p := Properties(o)
	if err := p.verify(); err != nil {
		return err
	}
	cfg.Properties = p
	return nil
}

func WithProperties(p Properties) WriterOption {
	return propertiesOption(p)
}

type fixedSizeOption int64

func (o fixedSizeOption) updateWriterConfig(cfg *writerConfig) error {
	if o < 0 {
		return fmt.Errorf("lzma: size must be non-negative")
	}
	cfg.FixedSize = true
	cfg.Size = int64(o)
	return nil
}

func WithFixedSize(size int64) WriterOption {
	return fixedSizeOption(size)
}

type pathFinderOption string

func (o pathFinderOption) updateWriterConfig(cfg *writerConfig) error {
	if o == "" {
		return fmt.Errorf("lzma: path finder must be non-empty")
	}
	cfg.PathFinder = string(o)
	return nil
}

func WithPathFinder(pathFinder string) WriterOption {
	return pathFinderOption(pathFinder)
}

type mapperOption string

func (o mapperOption) updateWriterConfig(cfg *writerConfig) error {
	if o == "" {
		return fmt.Errorf("lzma: mapper must be non-empty")
	}
	cfg.Mapper = string(o)
	return nil
}

func WithMapper(mapper string) WriterOption {
	return mapperOption(mapper)
}

type minMatchLenOption int

func (o minMatchLenOption) updateWriterConfig(cfg *writerConfig) error {
	if o < minMatchLen {
		return fmt.Errorf(
			"lzma: minimum match length must be at least %d",
			minMatchLen)
	}
	if o > maxMatchLen {
		return fmt.Errorf(
			"lzma: minimum match length must be at most %d",
			maxMatchLen)
	}
	cfg.MinMatchLen = int(o)
	return nil
}

func WithMinMatchLen(minMatchLen int) WriterOption {
	return minMatchLenOption(minMatchLen)
}

type maxMatchLenOption int

func (o maxMatchLenOption) updateWriterConfig(cfg *writerConfig) error {
	if o < minMatchLen {
		return fmt.Errorf(
			"lzma: maximum match length must be at least %d",
			minMatchLen)
	}
	if o > maxMatchLen {
		return fmt.Errorf(
			"lzma: maximum match length must be at most %d",
			maxMatchLen)
	}
	cfg.MaxMatchLen = int(o)
	return nil
}

// NewWriter creates a new LZMA writer.
func NewWriter(z io.Writer, options ...WriterOption) (w io.WriteCloser, err error) {
	cfg := writerConfig{
		WindowSize: 1 << 20,
		Properties: Properties{LC: 3, LP: 0, PB: 2},

		PathFinder: "greedy",
		Mapper:     "hash_3:16",

		MinMatchLen: minMatchLen,
		MaxMatchLen: maxMatchLen,
	}
	for _, opt := range options {
		if err = opt.updateWriterConfig(&cfg); err != nil {
			return nil, err
		}
	}

	parser, err := lz.NewParser(
		lz.WithWindowSize(cfg.WindowSize),
		lz.WithRetentionSize(cfg.WindowSize),
		lz.WithBufferSize(max(2*cfg.WindowSize, blockSize)),
		lz.WithPathFinder(cfg.PathFinder),
		lz.WithMapper(cfg.Mapper),
		lz.WithMinMatchLen(cfg.MinMatchLen),
		lz.WithMaxMatchLen(cfg.MaxMatchLen),
	)
	if err != nil {
		return nil, err
	}

	if err = cfg.Properties.verify(); err != nil {
		return nil, err
	}
	if !(0 <= cfg.WindowSize && cfg.WindowSize <= maxDictSize) {
		return nil, fmt.Errorf("lzma: window size must be between 0 and %d",
			maxDictSize)
	}

	p := Header{
		Properties: cfg.Properties,
		DictSize:   uint32(cfg.WindowSize),
	}
	if cfg.FixedSize {
		p.uncompressedSize = uint64(cfg.Size)
	} else {
		p.uncompressedSize = EOSSize
	}
	if err = p.verify(); err != nil {
		panic(err)
	}
	data, err := p.AppendBinary(nil)
	if err != nil {
		return nil, err
	}
	if _, err := z.Write(data); err != nil {
		return nil, err
	}

	if cfg.FixedSize {
		lw := &limitWriter{n: cfg.Size}
		if err := lw.w.init(z, parser, cfg.Properties, false); err != nil {
			return nil, err
		}
		return lw, nil
	}

	wr := new(writer)
	if err := wr.init(z, parser, cfg.Properties, true); err != nil {
		return nil, err
	}
	return wr, nil
}
