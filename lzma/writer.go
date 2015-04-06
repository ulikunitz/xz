package lzma

import (
	"io"

	"github.com/uli-go/xz/lzbase"
)

type Writer struct {
	lzbase.Writer
	params *Parameters
}

func NewWriter(w io.Writer) (lw *Writer, err error) {
	return NewWriterP(w, Default)
}

func NewWriterP(w io.Writer, p Parameters) (lw *Writer, err error) {
	if w == nil {
		return nil, newError("writer argument w is nil")
	}
	normalizeSizes(&p)
	if err = verifyParameters(&p); err != nil {
		return nil, err
	}
	if p.Size == 0 && !p.SizeInHeader {
		p.EOS = true
	}
	if err = writeHeader(w, &p); err != nil {
		return nil, err
	}
	dict, err := lzbase.NewWriterDict(p.DictSize, p.BufferSize)
	if err != nil {
		return nil, err
	}
	oc := lzbase.NewOpCodec(p.Properties(), dict)
	lw = new(Writer)
	if err = lzbase.InitWriter(&lw.Writer, w, oc,
		lzbase.Parameters{
			SizeInHeader: p.SizeInHeader,
			Size:         p.Size,
			EOS:          p.EOS}); err != nil {
		return nil, err
	}
	return lw, nil
}
