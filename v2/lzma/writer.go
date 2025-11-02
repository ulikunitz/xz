// SPDX-FileCopyrightText: © 2014 Ulrich Kunitz
//
// SPDX-License-Identifier: BSD-3-Clause

package lzma

import (
	"bufio"
	"errors"
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
	setUpdateEncoder(parser, func() *encoder {
		return &w.encoder
	})
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
		_, err := w.parser.Parse(&w.blk, 0, 0)
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
		w.parser.Prune(0)
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

// WriterOptions defines the parameters for the LZMA Writer.
type WriterOptions struct {
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

	// ParserOptions provides the LZ parser options. It defines which
	// parser will be used with what parameters.
	ParserOptions lz.ParserOptions
}

// Verify checks the validity of the writer configuration parameter.
func (opts *WriterOptions) verify() error {
	var err error

	if opts == nil {
		return errors.New("lzma: WriterConfig pointer must be non-nil")
	}

	if err = opts.Properties.Verify(); err != nil {
		return err
	}
	if opts.FixedSize && opts.Size < 0 {
		return errors.New("lzma: Size must be >= 0")
	}
	return nil
}

// SetDefaults applies the defaults to the configuration if they have not been
// set previously.
func (opts *WriterOptions) setDefaults() {
	if opts.WindowSize == 0 {
		opts.WindowSize = 8 << 20
	}
	var zeroProps = Properties{}
	if !opts.FixedProperties && opts.Properties == zeroProps {
		opts.Properties = Properties{3, 0, 2}
	}
}

// NewWriter creates a new LZMA writer.
func NewWriter(z io.Writer) (w io.WriteCloser, err error) {
	return NewWriterOptions(z, nil)
}

// NewWriterOptions creates a new LZMA writer using the parameter provided by
// cfg.
func NewWriterOptions(z io.Writer, opts *WriterOptions) (w io.WriteCloser, err error) {
	if opts == nil {
		opts = &WriterOptions{}
	}
	opts.setDefaults()
	if err = opts.verify(); err != nil {
		return nil, err
	}

	opts.ParserOptions.WindowSize = opts.WindowSize
	var parser lz.Parser
	if parser, err = lz.NewParser(&opts.ParserOptions); err != nil {
		return nil, err
	}

	p := Header{
		Properties: opts.Properties,
		DictSize:   uint32(opts.WindowSize),
	}
	if opts.FixedSize {
		p.uncompressedSize = uint64(opts.Size)
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

	if opts.FixedSize {
		lw := &limitWriter{n: opts.Size}
		lw.w.init(z, parser, opts.Properties, false)
		return lw, nil
	}

	wr := new(writer)
	wr.init(z, parser, opts.Properties, true)
	return wr, nil
}
