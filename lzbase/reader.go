package lzbase

import "io"

// Parameters provides a size if sizeInHeader is true. The size refers here to
// the uncompressed size.
type Parameters struct {
	Size         int64
	SizeInHeader bool
}

// Reader supports the reading of a raw LZMA stream without a header.
type Reader struct {
	OpCodec *OpCodec
	Dict    *ReaderDict
	rd      *rangeDecoder
	params  Parameters
}

// InitReader initializes the Reader. Note that the Dict field is taken from
// the OpCodec value.
func InitReader(br *Reader, r io.Reader, oc *OpCodec, params Parameters) error {
	switch {
	case r == nil:
		return newError("InitReader argument r is nil")
	case oc == nil:
		return newError("InitReader argument oc is nil")
	}
	dict, ok := oc.dict.(*ReaderDict)
	if !ok {
		return newError("op codec for reader expected")
	}
	rd, err := newRangeDecoder(r)
	if err != nil {
		return err
	}
	if params.SizeInHeader && params.Size < 0 {
		return newError("negative size parameter")
	}
	*br = Reader{OpCodec: oc, Dict: dict, rd: rd, params: params}
	return nil
}

// Reads reads data from the decoder stream.
//
// The method might block and is not reentrant.
//
// The end of the LZMA stream is indicated by EOF. There might be other errors
// returned. The decoder will not be able to recover from an error returned.
func (br *Reader) Read(p []byte) (n int, err error) {
	for {
		var k int
		k, err = br.Dict.Read(p[n:])
		n += k
		switch {
		case err == io.EOF:
			if n <= 0 {
				return 0, io.EOF
			}
			return n, nil
		case err != nil:
			return n, err
		case n == len(p):
			return n, nil
		}
		if err = br.fill(); err != nil {
			return n, err
		}
	}
}

// decodeLiteral reads a literal
func (br *Reader) decodeLiteral() (op operation, err error) {
	oc := br.OpCodec
	litState := oc.litState()

	match := br.Dict.Byte(int64(oc.rep[0]) + 1)
	s, err := oc.litCodec.Decode(br.rd, oc.state, match, litState)
	if err != nil {
		return nil, err
	}
	return lit{s}, nil
}

// Indicates that the end of stream marker has been unexpected.
var errUnexpectedEOS = newError("unexpected end-of-stream marker")

// errWrongTermination indicates that a termination symbol has been received,
// but the range decoder could still produces more data
var errWrongTermination = newError("end of stream marker at wrong place")

// eos indicates an explicit end of stream
var eos = newError("end of decoded stream")

// readOp decodes the next operation from the compressed stream. It returns the
// operation. If an exlicit end of stream marker is identified the eos error is
// returned.
func (br *Reader) readOp() (op operation, err error) {
	oc := br.OpCodec
	state, state2, posState := oc.states()

	b, err := oc.isMatch[state2].Decode(br.rd)
	if err != nil {
		return nil, err
	}
	if b == 0 {
		// literal
		op, err := br.decodeLiteral()
		if err != nil {
			return nil, err
		}
		oc.updateStateLiteral()
		return op, nil
	}
	b, err = oc.isRep[state].Decode(br.rd)
	if err != nil {
		return nil, err
	}
	if b == 0 {
		// simple match
		oc.rep[3], oc.rep[2], oc.rep[1] = oc.rep[2], oc.rep[1], oc.rep[0]

		oc.updateStateMatch()
		// The length decoder returns the length offset.
		n, err := oc.lenCodec.Decode(br.rd, posState)
		if err != nil {
			return nil, err
		}
		// The dist decoder returns the distance offset. The actual
		// distance is 1 higher.
		oc.rep[0], err = oc.distCodec.Decode(br.rd, n)
		if err != nil {
			return nil, err
		}
		if oc.rep[0] == eosDist {
			if !br.rd.possiblyAtEnd() {
				return nil, errWrongTermination
			}
			return nil, eos
		}
		op = match{length: int(n) + MinLength,
			distance: int64(oc.rep[0]) + minDistance}
		return op, nil
	}
	b, err = oc.isRepG0[state].Decode(br.rd)
	if err != nil {
		return nil, err
	}
	dist := oc.rep[0]
	if b == 0 {
		// rep match 0
		b, err = oc.isRepG0Long[state2].Decode(br.rd)
		if err != nil {
			return nil, err
		}
		if b == 0 {
			oc.updateStateShortRep()
			op = match{length: 1,
				distance: int64(dist) + minDistance}
			return op, nil
		}
	} else {
		b, err = oc.isRepG1[state].Decode(br.rd)
		if err != nil {
			return nil, err
		}
		if b == 0 {
			dist = oc.rep[1]
		} else {
			b, err = oc.isRepG2[state].Decode(br.rd)
			if err != nil {
				return nil, err
			}
			if b == 0 {
				dist = oc.rep[2]
			} else {
				dist = oc.rep[3]
				oc.rep[3] = oc.rep[2]
			}
			oc.rep[2] = oc.rep[1]
		}
		oc.rep[1] = oc.rep[0]
		oc.rep[0] = dist
	}
	n, err := oc.repLenCodec.Decode(br.rd, posState)
	if err != nil {
		return nil, err
	}
	oc.updateStateRep()
	op = match{length: int(n) + MinLength,
		distance: int64(dist) + minDistance}
	return op, nil
}

// fill puts at lest the requested number of bytes into the decoder Dictionary.
func (br *Reader) fill() error {
	if br.Dict.closed {
		return nil
	}
	for br.Dict.Writable() >= MaxLength {
		op, err := br.readOp()
		if err != nil {
			switch {
			case err == eos:
				if br.params.SizeInHeader &&
					br.Dict.Offset() != br.params.Size {
					return errUnexpectedEOS
				}
				br.Dict.closed = true
				if !br.rd.possiblyAtEnd() {
					return newError("data after eos")
				}
				return nil
			case err == io.EOF:
				return newError(
					"unexpected end of compressed stream")
			default:
				return err
			}
		}
		debug.Printf("op %s", op)

		if err = op.applyReaderDict(br.Dict); err != nil {
			return err
		}
		if br.params.SizeInHeader && br.Dict.Offset() >= br.params.Size {
			if br.Dict.Offset() > br.params.Size {
				return newError(
					"more data than announced in header")
			}
			br.Dict.closed = true
			if !br.rd.possiblyAtEnd() {
				if _, err = br.readOp(); err != eos {
					return newError(
						"wrong length in header")
				}
				if !br.rd.possiblyAtEnd() {
					return newError("data after eos")
				}
			}
			return nil
		}
	}
	return nil
}
