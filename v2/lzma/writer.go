// SPDX-FileCopyrightText: © 2014 Ulrich Kunitz
//
// SPDX-License-Identifier: BSD-3-Clause

package lzma

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"io"

	"github.com/ulikunitz/lz"
)

func setUpdateEncoder(p lz.Parser, updateEncoder func() *encoder) {
	/* TODO
	   op, ok := p.(*optParser)
	   if !ok {
	           return
	   }
	   op.updateEncoder = updateEncoder
	*/
}

// NewRawWriter writes only the compressed data stream. The argument eos
// controls whether an end of stream marker will be written.
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

// init initializes a writer. The eos argument tells the writer whether an
// end-of-stream marker should be written.
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

// Close closes the writer.
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

// WriterConfig defines the parameters for the LZMA Writer.
// TODO: add comments
type WriterConfig struct {
	// WindowSize defines the size of the sliding dictionary window for the
	// LZ parsing. If it is non-zero it overrides the parser configuration
	// of the lz package.
	WindowSize int `json:",omitzero"`

	// BufferSize defines the size of the buffer for the LZ parsing.
	BufferSize int `json:",omitzero"`

	// Properties of the LZMA algorithm.
	Properties *Properties `json:",omitzero"`

	FixedSize *int64 `json:",omitzero"`

	PathFinder string `json:",omitzero"`
	Mapper     string `json:",omitzero"`
}

type writerJSONConfig struct {
	Format     string
	WindowSize int         `json:",omitzero"`
	BufferSize int         `json:",omitzero"`
	Properties *Properties `json:",omitzero"`
	FixedSize  *int64      `json:",omitzero"`
	PathFinder string      `json:",omitzero"`
	Mapper     string      `json:",omitzero"`
}

func (cfg *WriterConfig) UnmarshalJSON(data []byte) error {
	var c writerJSONConfig
	if err := json.Unmarshal(data, &c); err != nil {
		return err
	}
	if c.Format != "lzma" {
		return fmt.Errorf("lzma: invalid format %q", c.Format)
	}
	*cfg = WriterConfig{
		WindowSize: c.WindowSize,
		BufferSize: c.BufferSize,
		PathFinder: c.PathFinder,
		Mapper:     c.Mapper,
	}
	if c.Properties != nil {
		cfg.Properties = c.Properties
	}
	if c.FixedSize != nil {
		cfg.FixedSize = c.FixedSize
	}
	return nil
}

func (cfg *WriterConfig) MarshalJSON() ([]byte, error) {
	c := writerJSONConfig{
		Format:     "lzma",
		WindowSize: cfg.WindowSize,
		BufferSize: cfg.BufferSize,
		Properties: cfg.Properties,
		FixedSize:  cfg.FixedSize,
		PathFinder: cfg.PathFinder,
		Mapper:     cfg.Mapper,
	}
	return json.Marshal(&c)
}

// Verify checks the validity of the writer configuration parameter.
func (cfg *WriterConfig) verify() error {
	var err error

	if cfg == nil {
		return errors.New("lzma: WriterConfig pointer must be non-nil")
	}

	if !(minWindowSize <= cfg.WindowSize && int64(cfg.WindowSize) <= maxWindowSize) {
		return fmt.Errorf("lzma: WindowSize must be between %d and %d",
			minWindowSize, maxWindowSize)
	}

	if cfg.Properties == nil {
		return errors.New("lzma: Properties must be set")
	}

	if err = cfg.Properties.verify(); err != nil {
		return err
	}

	if cfg.FixedSize != nil {
		if *cfg.FixedSize < 0 {
			return errors.New(
				"lzma: FixedSize must be non-negative")
		}
	}

	if cfg.PathFinder == "" {
		return errors.New("lzma: path finder must be set")
	}

	if cfg.Mapper == "" {
		return errors.New("lzma: mapper must be set")
	}

	return nil
}

// SetDefaults applies defaults to the configuration if they have not been
// set previously.
func (cfg *WriterConfig) SetDefaults() {
	if cfg.WindowSize == 0 {
		cfg.WindowSize = 8 << 20
	}
	if cfg.BufferSize == 0 {
		cfg.BufferSize = 2 * cfg.WindowSize
	}

	if cfg.Properties == nil {
		cfg.Properties = &Properties{3, 0, 2}
	}

	if cfg.PathFinder == "" {
		cfg.PathFinder = "greedy"
	}

	if cfg.Mapper == "" {
		cfg.Mapper = "hash_2:16"
	}
}

// clone returns a deep copy of the writer configuration.
func (cfg WriterConfig) clone() WriterConfig {
	c := cfg
	if cfg.Properties != nil {
		c.Properties = new(*cfg.Properties)
	}
	return c
}

// NewWriter creates a new LZMA writer.
func NewWriter(z io.Writer) (w io.WriteCloser, err error) {
	return NewWriterConfig(z, WriterConfig{})
}

// NewWriterConfig creates a new LZMA writer using the parameter provided by
// options.
func NewWriterConfig(z io.Writer, cfg WriterConfig) (w io.WriteCloser, err error) {
	cfg = cfg.clone()
	cfg.SetDefaults()
	if err = cfg.verify(); err != nil {
		return nil, err
	}

	lzCfg := lz.ParserConfig{
		WindowSize:    new(cfg.WindowSize),
		RetentionSize: new(cfg.WindowSize),
		BufferSize:    cfg.BufferSize,
		PathFinder:    cfg.PathFinder,
		Mapper:        cfg.Mapper,
		MinMatchLen:   2,
		MaxMatchLen:   273,
	}
	parser, err := lz.NewParser(lzCfg)
	if err != nil {
		return nil, err
	}

	p := Header{
		Properties: *cfg.Properties,
		DictSize:   uint32(cfg.WindowSize),
	}
	if cfg.FixedSize != nil {
		p.uncompressedSize = uint64(*cfg.FixedSize)
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

	if cfg.FixedSize != nil {
		lw := &limitWriter{n: *cfg.FixedSize}
		if err := lw.w.init(z, parser, *cfg.Properties, false); err != nil {
			return nil, err
		}
		return lw, nil
	}

	wr := new(writer)
	if err := wr.init(z, parser, *cfg.Properties, true); err != nil {
		return nil, err
	}
	return wr, nil
}
