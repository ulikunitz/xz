package lzma

import (
	"bufio"
	"errors"
	"fmt"
	"io"

	"github.com/ulikunitz/lz"
)

const (
	// mb give the number of bytes in a megabyte.
	mb = 1 << 20
)

// minDictSize defines the minumum supported dictionary size.
const minDictSize = 1 << 12

// ErrUnexpectedEOS reports an unexpected end-of-stream marker
var ErrUnexpectedEOS = errors.New("lzma: unexpected end of stream")

// ErrEncoding reports an encoding error
var ErrEncoding = errors.New("lzma: wrong encoding")

// NewReader creates a reader for LZMA-compressed streams. It reads the LZTMA
// header and creates a reader and may return an error if the header is wrong.
func NewReader(z io.Reader) (r io.Reader, err error) {
	headerBuf := make([]byte, headerLen)
	if _, err = io.ReadFull(z, headerBuf); err != nil {
		if err == io.EOF {
			err = io.ErrUnexpectedEOF
		}
		return nil, err
	}

	var p params
	if err = p.UnmarshalBinary(headerBuf); err != nil {
		return nil, err
	}
	if p.dictSize < minDictSize {
		p.dictSize = minDictSize
	}
	if err = p.Verify(); err != nil {
		return nil, err
	}

	t := new(reader)
	if err = t.init(z, p); err != nil {
		return nil, err
	}

	return t, nil
}

// WriterConfig provides configuration parameters for the LZMA writer.
type WriterConfig struct {
	Properties
	// set to true if you want LC, LP and PB actuially zero
	PropertiesInitialized bool
	DictSize              int
	MemoryBudget          int
	Effort                int
}

func (cfg *WriterConfig) Verify() error {
	var err error
	if err = cfg.Properties.Verify(); err != nil {
		return err
	}
	if !(1 <= cfg.Effort && cfg.Effort <= 9) {
		return fmt.Errorf("lzma: effort out of range 1..9")
	}
	if !(0 <= cfg.DictSize && cfg.DictSize <= maxDistance) {
		return fmt.Errorf("lzma: DictSize out of range")
	}
	if !(cfg.DictSize <= cfg.MemoryBudget) {
		return fmt.Errorf("lzma: MemBudget must be larget then DictSize")
	}
	return nil
}

func (cfg *WriterConfig) ApplyDefaults() {
	if !cfg.PropertiesInitialized {
		if cfg.Properties.LC == 0 && cfg.Properties.LP == 0 &&
			cfg.Properties.PB == 0 {
			cfg.Properties = Properties{
				LC: 2,
				LP: 0,
				PB: 3,
			}
		}
	}
	lzcfg := lz.Config{
		MemoryBudget: cfg.MemoryBudget,
		WindowSize:   cfg.DictSize,
		Effort:       cfg.Effort,
	}
	lzcfg.ApplyDefaults()
	cfg.MemoryBudget = lzcfg.MemoryBudget
	cfg.DictSize = lzcfg.WindowSize
	cfg.Effort = lzcfg.Effort
}

// NewWriter creates a single-threaded writer of LZMA files.
func NewWriter(z io.Writer) (w io.WriteCloser, err error) {
	cfg := WriterConfig{
		Properties:            Properties{LC: 3, LP: 0, PB: 2},
		PropertiesInitialized: true,
		DictSize:              8 * mb,
		MemoryBudget:          10 * mb,
		Effort:                5,
	}
	return NewWriterConfig(z, cfg)
}

// NewWriterConfig creates a new writer generating LZMA files.
func NewWriterConfig(z io.Writer, cfg WriterConfig) (w io.WriteCloser, err error) {
	cfg.ApplyDefaults()
	if err = cfg.Verify(); err != nil {
		return nil, err
	}
	lzcfg := lz.Config{
		MemoryBudget: cfg.MemoryBudget,
		WindowSize:   cfg.DictSize,
		Effort:       cfg.Effort,
	}
	lzcfg.ApplyDefaults()
	seq, err := lzcfg.NewSequencer()
	if err != nil {
		return nil, err
	}
	wr := &writer{
		seq: seq,
		w:   seq.WindowPtr(),
		bw:  bufio.NewWriter(z),
	}
	h := params{
		p:                cfg.Properties,
		dictSize:         uint32(wr.w.WindowSize),
		uncompressedSize: eosSize,
	}

	hdr, err := h.AppendBinary(nil)
	if err != nil {
		return nil, err
	}
	if _, err = wr.bw.Write(hdr); err != nil {
		return nil, err
	}

	wr.eos = true
	wr.e.init(wr.bw, wr.w, cfg.Properties)
	return wr, nil
}

type writer struct {
	seq lz.Sequencer
	w   *lz.Window
	e   encoder
	blk lz.Block
	eos bool
	err error
	bw  *bufio.Writer
}

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
				err = w.e.writeLiteral(c)
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
			err = w.e.writeMatch(s.Offset-1, s.MatchLen)
			if err != nil {
				return err
			}
		}
		for _, c := range w.blk.Literals[litIndex:] {
			err = w.e.writeLiteral(c)
			if err != nil {
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
		k, err := w.w.Write(p[n:])
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

func (w *writer) Close() error {
	if w.err != nil {
		return w.err
	}
	err := w.clearBuffer()
	if err != nil {
		w.err = err
		return err
	}
	if w.eos {
		err = w.e.writeMatch(eosDist, minMatchLen)
		if err != nil {
			w.err = err
			return err
		}
	}
	err = w.e.Close()
	if err != nil {
		w.err = err
		return err
	}
	err = w.bw.Flush()
	if err != nil {
		w.err = err
		return err
	}
	w.err = errClosed
	return nil
}
