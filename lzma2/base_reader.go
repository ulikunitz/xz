package lzma2

import "io"

type baseReader struct {
	opCodec *opCodec
	dict    *readerDict
	rd      *rangeDecoder
}

func newBaseReader(r io.Reader, opCodec *opCodec) (br *baseReader, err error) {
	switch {
	case r == nil:
		return nil, newError("newBaseReader argument r is nil")
	case opCodec == nil:
		return nil, newError("newBaseReader argument opCodec is nil")
	}
	dict, ok := opCodec.dict.(*readerDict)
	if !ok {
		return nil, newError("op codec for reader expected")
	}
	rd, err := newRangeDecoder(r)
	if err != nil {
		return nil, err
	}
	return &baseReader{
		opCodec: opCodec,
		dict:    dict,
		rd:      rd}, nil
}
