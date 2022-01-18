package lzma

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"math"

	"github.com/ulikunitz/lz"
)

// NewRawWriter writes only compress data stream. The argument eos controls
// whether an end of stream marker will be written.
func NewRawWriter(z io.Writer, seq lz.Sequencer, p Properties,
	eos bool) (w io.WriteCloser, err error) {

	if err = p.Verify(); err != nil {
		return nil, err
	}

	wr := new(writer)
	wr.init(z, seq, p, eos)
	return wr, nil
}

// writer is a helper structure to implement writers. It provides the
// writeMatch and writeLiteral functions.
type writer struct {
	seq    lz.Sequencer
	window *lz.Window
	pos    int64
	state  state
	re     rangeEncoder
	blk    lz.Block
	eos    bool
	err    error
	bufw   *bufio.Writer
}

func (w *writer) init(z io.Writer, seq lz.Sequencer, p Properties, eos bool) {
	var bufw *bufio.Writer
	bw, ok := z.(io.ByteWriter)
	if !ok {
		bufw = bufio.NewWriter(z)
		bw = bufw
	}

	*w = writer{
		seq:    seq,
		window: seq.WindowPtr(),
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

func (w *writer) byteAtEnd(i int64) byte {
	c, _ := w.window.ReadByteAt(w.pos - i)
	return c
}

// writeLiteral encodes a single literal byte.
func (w *writer) writeLiteral(c byte) error {
	state, state2, _ := w.state.states(w.pos)
	var err error
	if err = w.re.EncodeBit(0, &w.state.s2[state2].isMatch); err != nil {
		return err
	}
	litState := w.state.litState(w.byteAtEnd(1), w.pos)
	match := w.byteAtEnd(int64(w.state.rep[0]) + 1)
	err = w.state.litCodec.Encode(&w.re, c, state, match, litState)
	if err != nil {
		return err
	}
	w.state.updateStateLiteral()
	w.pos++
	return nil
}

func iverson(f bool) uint32 {
	if f {
		return 1
	}
	return 0
}

// writeMatch writes a match. The argument dist equals offset - 1.
func (w *writer) writeMatch(dist, matchLen uint32) error {
	var err error

	if !(minMatchLen <= matchLen && matchLen <= maxMatchLen) &&
		!(dist == w.state.rep[0] && matchLen == 1) {
		return fmt.Errorf(
			"match length %d out of range; dist %d rep[0] %d",
			matchLen, dist, w.state.rep[0])
	}
	state, state2, posState := w.state.states(w.pos)
	if err = w.re.EncodeBit(1, &w.state.s2[state2].isMatch); err != nil {
		return err
	}
	g := 0
	for ; g < 4; g++ {
		if w.state.rep[g] == dist {
			break
		}
	}
	b := iverson(g < 4)
	if err = w.re.EncodeBit(b, &w.state.s1[state].isRep); err != nil {
		return err
	}
	n := matchLen - minMatchLen
	if b == 0 {
		// simple match
		w.state.rep[3], w.state.rep[2], w.state.rep[1], w.state.rep[0] =
			w.state.rep[2], w.state.rep[1], w.state.rep[0], dist
		w.state.updateStateMatch()
		if err = w.state.lenCodec.Encode(&w.re, n, posState); err != nil {
			return err
		}
		if err = w.state.distCodec.Encode(&w.re, dist, n); err != nil {
			return err
		}
		w.pos += int64(matchLen)
		return nil
	}
	b = iverson(g != 0)
	if err = w.re.EncodeBit(b, &w.state.s1[state].isRepG0); err != nil {
		return err
	}
	if b == 0 {
		// g == 0
		b = uint32(iverson(matchLen != 1))
		if err = w.re.EncodeBit(b, &w.state.s2[state2].isRepG0Long); err != nil {
			return err
		}
		if b == 0 {
			w.state.updateStateShortRep()
			w.pos++
			return nil
		}
	} else {
		// g in {1,2,3}
		b = uint32(iverson(g != 1))
		if err = w.re.EncodeBit(b, &w.state.s1[state].isRepG1); err != nil {
			return err
		}
		if b == 1 {
			// g in {2,3}
			b = iverson(g != 2)
			err = w.re.EncodeBit(b, &w.state.s1[state].isRepG2)
			if err != nil {
				return err
			}
			if b == 1 {
				w.state.rep[3] = w.state.rep[2]
			}
			w.state.rep[2] = w.state.rep[1]
		}
		w.state.rep[1] = w.state.rep[0]
		w.state.rep[0] = dist
	}
	w.state.updateStateRep()
	if err = w.state.repLenCodec.Encode(&w.re, n, posState); err != nil {
		return err
	}
	w.pos += int64(matchLen)
	return nil
}

var errClosed = errors.New("lzma: already closed")

func (w *writer) clearBuffer() error {
	for {
		_, err := w.seq.Sequence(&w.blk, 0)
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
		w.seq.Shrink()
	}
}

type limitWriter struct {
	n int64
	w writer
}

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

func (lw *limitWriter) Close() error {
	if lw.n > 0 {
		return errors.New("lzma: more data required")
	}
	return lw.w.Close()
}

type WriterConfig struct {
	LZCfg      lz.Configurator
	Properties Properties
	ZeroProps  bool
	FixedSize  bool
	Size       int64
}

func (cfg *WriterConfig) Verify() error {
	if cfg.LZCfg == nil {
		return errors.New("lzma: no lz configuration provided")
	}
	if err := cfg.Properties.Verify(); err != nil {
		return err
	}
	if cfg.FixedSize && cfg.Size < 0 {
		return errors.New("lzma: Size must be >= 0")
	}
	return nil
}

func (cfg *WriterConfig) ApplyDefaults() {
	if cfg.LZCfg == nil {
		var c lz.Config
		c.ApplyDefaults()
		cfg.LZCfg = &c
	}
	var emptyProps = Properties{}
	if cfg.Properties == emptyProps && !cfg.ZeroProps {
		cfg.Properties = Properties{3, 0, 2}
	}
}

func NewWriter(z io.Writer) (w io.WriteCloser, err error) {
	return NewWriterConfig(z, WriterConfig{})
}

func NewWriterConfig(z io.Writer, cfg WriterConfig) (w io.WriteCloser, err error) {
	cfg.ApplyDefaults()
	if err = cfg.Verify(); err != nil {
		return nil, err
	}

	var seq lz.Sequencer
	if seq, err = cfg.LZCfg.NewSequencer(); err != nil {
		return nil, err
	}

	window := seq.WindowPtr()
	dictSize := int64(window.WindowSize)
	if !(0 <= dictSize && dictSize <= math.MaxUint32) {
		return nil, errors.New("lzma: dictSize out of range")
	}
	p := params{
		props:    cfg.Properties,
		dictSize: uint32(dictSize),
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
		lw.w.init(z, seq, cfg.Properties, false)
		return lw, nil
	}

	wr := new(writer)
	wr.init(z, seq, cfg.Properties, true)
	return wr, nil
}
