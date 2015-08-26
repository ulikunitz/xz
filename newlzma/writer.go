package newlzma

import (
	"errors"
	"io"
)

type Flags int

const (
	EOS Flags = 1 << iota
	NoUncompressedSize
)

const (
	MinDictCap = 1 << 12
	MaxDictCap = 1<<32 - 1
)

type Params struct {
	LC               int
	LP               int
	PB               int
	DictCap          int
	UncompressedSize int64
	Flags            Flags
}

func verifyParams(p *Params) error {
	if err := verifyProperties(p.LC, p.LP, p.PB); err != nil {
		return err
	}
	if !(MinDictCap <= p.DictCap && p.DictCap <= MaxDictCap) {
		return errors.New("DictCap out of range")
	}
	if p.Flags&NoUncompressedSize == 0 {
		if p.UncompressedSize < 0 {
			return errors.New(
				"UncompressedSize must be greate or equal 0")
		}
	} else {
		if p.Flags&EOS == 0 {
			return errors.New("if no uncompressed size is given, " +
				"EOS must be set")
		}
	}
	return nil
}

var Default = Params{
	LC:      3,
	LP:      0,
	PB:      2,
	DictCap: 8 * 1024 * 1024,
	Flags:   EOS | NoUncompressedSize,
}

func convertParams(p *Params) CodecParams {
	c := CodecParams{
		DictCap:          p.DictCap,
		BufCap:           p.DictCap + 1<<13,
		UncompressedSize: p.UncompressedSize,
		LC:               p.LC,
		LP:               p.LP,
		PB:               p.PB,
		Flags:            CNoCompressedSize,
	}
	if p.Flags&EOS != 0 {
		c.Flags |= CEOSMarker
	}
	if p.Flags&NoUncompressedSize != 0 {
		c.Flags |= CNoUncompressedSize
	}
	return c
}

type Writer struct {
	e Encoder
}

func NewWriter(lzma io.Writer) (w *Writer, err error) {
	return NewWriterParams(lzma, &Default)
}

func NewWriterParams(lzma io.Writer, p *Params) (w *Writer, err error) {
	if err = verifyParams(p); err != nil {
		return nil, err
	}
	cparams := convertParams(p)
	if err := writeHeader(lzma, &cparams); err != nil {
		return nil, err
	}
	w = new(Writer)
	if err = InitEncoder(&w.e, lzma, &cparams); err != nil {
		return nil, err
	}
	return w, nil
}

func (w *Writer) Write(p []byte) (n int, err error) {
	return w.e.Write(p)
}

func (w *Writer) Close() error {
	if w.e.flags&CNoUncompressedSize == 0 {
		if w.e.Uncompressed()+int64(w.e.Buffered()) != w.e.uncompressedSize {
			return ErrUncompressedSize
		}
	}
	return w.e.Close()
}
