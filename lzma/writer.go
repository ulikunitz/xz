package lzma

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"

	"github.com/ulikunitz/lz"
)

// NewRawWriter writes only compress data stream. The argument eos controls
// whether an end of stream marker will be written.
func NewRawWriter(z io.Writer, parser lz.Parser, p Properties, eos bool) (w io.WriteCloser, err error) {

	if err = p.Verify(); err != nil {
		return nil, err
	}

	wr := new(writer)
	wr.init(z, parser, p, eos)
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
func (w *writer) init(z io.Writer, parser lz.Parser, p Properties, eos bool) {
	var bufw *bufio.Writer
	bw, ok := z.(io.ByteWriter)
	if !ok {
		bufw = bufio.NewWriter(z)
		bw = bufw
	}

	*w = writer{
		parser:  parser,
		encoder: encoder{window: parser},
		blk: lz.Block{
			Sequences: w.blk.Sequences[:0],
			Literals:  w.blk.Literals[:0],
		},

		bufw: bufw,
		eos:  eos,
	}

	w.state.init(p)
	w.re.init(bw)
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
		_, err := w.parser.Parse(&w.blk, 0)
		if err != nil {
			if err == lz.ErrEmptyBuffer {
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
		k, err := w.window.Write(p[n:])
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
		w.parser.Shrink()
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
type WriterConfig struct {
	// WindowSize defines the size of the sliding dictionary window for the
	// LZ parsing. If it is non-zero it overrides the parser configuration
	// of the lz package.
	WindowSize int

	// Properties of the LZMA algorithm.
	Properties Properties

	// If true the properties are actually zero.
	FixedProperties bool

	// FixedSize says that the stream has a fixed size known before
	// compression.
	FixedSize bool

	// Size gives the actual size if FixedSize is set.
	Size int64

	// ParserConfig provides the LZ parser configuration. It defines which
	// parser will be used with what parameters note that the WindowSize
	// overrules the WindowSize in the parser configuration. Note also that
	// ShrinkSize must have WindowSize for the LZMA algorithm.
	ParserConfig lz.ParserConfig
}

func (cfg *WriterConfig) UnmarshalJSON(p []byte) error {
	var err error
	s := struct {
		Format          string
		Type            string
		WindowSize      int             `json:",omitempty"`
		LC              int             `json:",omitempty"`
		LP              int             `json:",omitempty"`
		PB              int             `json:",omitempty"`
		FixedProperties bool            `json:",omitempty"`
		FixedSize       bool            `json:",omitempty"`
		Size            int64           `json:",omitempty"`
		ParserConfig    json.RawMessage `json:",omitempty"`
	}{}
	if err = json.Unmarshal(p, &s); err != nil {
		return err
	}
	if s.Format != "LZMA" {
		return errors.New(
			"lzma: Format JSON property muse have value LZMA")
	}
	if s.Type != "Writer" {
		return errors.New(
			"lzma: Type JSON property must have value Writer")
	}
	parserConfig, err := lz.ParseJSON(s.ParserConfig)
	if err != nil {
		return fmt.Errorf("lzma.WriterConfig.UnmarshalJSON: %w", err)
	}
	*cfg = WriterConfig{
		WindowSize: s.WindowSize,
		Properties: Properties{
			LC: s.LC,
			LP: s.LP,
			PB: s.PB,
		},
		FixedProperties: s.FixedProperties,
		FixedSize:       s.FixedSize,
		Size:            s.Size,
		ParserConfig:    parserConfig,
	}
	return nil
}

func (cfg *WriterConfig) MarshalJSON() (p []byte, err error) {
	s := struct {
		Format          string
		Type            string
		WindowSize      int             `json:",omitempty"`
		LC              int             `json:",omitempty"`
		LP              int             `json:",omitempty"`
		PB              int             `json:",omitempty"`
		FixedProperties bool            `json:",omitempty"`
		FixedSize       bool            `json:",omitempty"`
		Size            int64           `json:",omitempty"`
		ParserConfig    lz.ParserConfig `json:",omitempty"`
	}{
		Format:          "LZMA",
		Type:            "Writer",
		WindowSize:      cfg.WindowSize,
		LC:              cfg.Properties.LC,
		LP:              cfg.Properties.LP,
		PB:              cfg.Properties.PB,
		FixedProperties: cfg.FixedProperties,
		FixedSize:       cfg.FixedSize,
		Size:            cfg.Size,
		ParserConfig:    cfg.ParserConfig,
	}
	return json.Marshal(&s)
}

// Verify checks the validity of the writer configuration parameter.
func (cfg *WriterConfig) Verify() error {
	var err error

	if cfg == nil {
		return errors.New("lzma: WriterConfig pointer must be non-nil")
	}

	if cfg.ParserConfig == nil {
		return errors.New("lzma: no LZ parser configuration provided")
	}
	if err = cfg.ParserConfig.Verify(); err != nil {
		return err
	}

	if err = cfg.Properties.Verify(); err != nil {
		return err
	}
	if cfg.FixedSize && cfg.Size < 0 {
		return errors.New("lzma: Size must be >= 0")
	}
	return nil
}

// SetDefaults applies the defaults to the configuration if they have not been
// set previously.
func (cfg *WriterConfig) SetDefaults() {
	if cfg.ParserConfig == nil {
		cfg.ParserConfig = &lz.DHPConfig{}
	}
	if cfg.WindowSize == 0 {
		cfg.ParserConfig.SetDefaults()
		cfg.WindowSize = cfg.ParserConfig.BufConfig().WindowSize
	}
	fixBufConfig(cfg.ParserConfig, cfg.WindowSize)
	cfg.ParserConfig.SetDefaults()

	var zeroProps = Properties{}
	if !cfg.FixedProperties && cfg.Properties == zeroProps {
		cfg.Properties = Properties{3, 0, 2}
	}
}

// NewWriter creates a new LZMA writer.
func NewWriter(z io.Writer) (w io.WriteCloser, err error) {
	return NewWriterConfig(z, WriterConfig{})
}

// NewWriterConfig creates a new LZMA writer using the parameter provided by
// cfg.
func NewWriterConfig(z io.Writer, cfg WriterConfig) (w io.WriteCloser, err error) {
	cfg.SetDefaults()
	if err = cfg.Verify(); err != nil {
		return nil, err
	}

	var parser lz.Parser
	if parser, err = cfg.ParserConfig.NewParser(); err != nil {
		return nil, err
	}

	windowSize := int64(parser.BufferConfig().WindowSize)
	if !(0 <= windowSize && windowSize <= math.MaxUint32) {
		return nil, errors.New("lzma: dictSize out of range")
	}
	p := params{
		props:    cfg.Properties,
		dictSize: uint32(windowSize),
	}
	if cfg.FixedSize {
		p.uncompressedSize = uint64(cfg.Size)
	} else {
		p.uncompressedSize = EOSSize
	}
	if err = p.Verify(); err != nil {
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
		lw.w.init(z, parser, cfg.Properties, false)
		return lw, nil
	}

	wr := new(writer)
	wr.init(z, parser, cfg.Properties, true)
	return wr, nil
}
