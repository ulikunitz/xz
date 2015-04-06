package lzbase

import "io"

type Writer struct {
	OpCodec *OpCodec
	Dict    *WriterDict
	re      *rangeEncoder
}

func InitWriter(bw *Writer, w io.Writer, oc *OpCodec) (err error) {
	switch {
	case w == nil:
		return newError("InitWriter argument w is nil")
	case oc == nil:
		return newError("InitWriter argument oc is nil")
	}
	dict, ok := oc.dict.(*WriterDict)
	if !ok {
		return newError("op codec for writer expected")
	}
	re := newRangeEncoder(w)
	*bw = Writer{OpCodec: oc, Dict: dict, re: re}
	return nil
}
