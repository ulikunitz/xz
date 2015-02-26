package lzma

import (
	"io"
)

// noHeaderLen defines the value of the length field in the LZMA header.
const noHeaderLen uint64 = 1<<64 - 1

// Reader supports the reading of LZMA byte streams.
type Reader struct {
	opCodec
	dict   *readerDict
	rd     *rangeDecoder
	params *Parameters
}

// NewReader creates a reader for LZMA byte streams. It reads the LZMA file
// header.
//
// For high performance use a buffered reader.
func NewReader(r io.Reader) (*Reader, error) {
	p, err := readHeader(r)
	if err != nil {
		return nil, err
	}
	if err = verifyParameters(p); err != nil {
		return nil, err
	}
	lr := &Reader{params: p}
	lr.dict, err = newReaderDict(int(p.DictSize), p.BufferSize)
	if err != nil {
		return nil, err
	}
	if lr.rd, err = newRangeDecoder(r); err != nil {
		return nil, err
	}
	lr.opCodec.init(p.Properties(), lr.dict)
	return lr, nil
}

// readUint64LE reads a uint64 little-endian integer from reader.
func readUint64LE(r io.Reader) (x uint64, err error) {
	b := make([]byte, 8)
	if _, err = io.ReadFull(r, b); err != nil {
		return 0, err
	}
	x = getUint64LE(b)
	return x, nil
}

// Reads reads data from the decoder stream.
//
// The method might block and is not reentrant.
//
// The end of the LZMA stream is indicated by EOF. There might be other errors
// returned. The decoder will not be able to recover from an error returned.
func (lr *Reader) Read(p []byte) (n int, err error) {
	for {
		var k int
		k, err = lr.dict.Read(p[n:])
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
		if err = lr.fill(); err != nil {
			return n, err
		}
	}
}

// decodeLiteral reads a literal
func (lr *Reader) decodeLiteral() (op operation, err error) {
	litState := lr.litState()

	match := lr.dict.Byte(int(lr.rep[0]) + 1)
	s, err := lr.litCodec.Decode(lr.rd, lr.state, match, litState)
	if err != nil {
		return nil, err
	}
	return lit{s}, nil
}

// eos indicates an explicit end of stream
var eos = newError("end of decoded stream")

// readOp decodes the next operation from the compressed stream. It returns the
// operation. If an exlicit end of stream marker is identified the eos error is
// returned.
func (lr *Reader) readOp() (op operation, err error) {
	state, state2, posState := lr.states()

	b, err := lr.isMatch[state2].Decode(lr.rd)
	if err != nil {
		return nil, err
	}
	if b == 0 {
		// literal
		op, err := lr.decodeLiteral()
		if err != nil {
			return nil, err
		}
		lr.updateStateLiteral()
		return op, nil
	}
	b, err = lr.isRep[state].Decode(lr.rd)
	if err != nil {
		return nil, err
	}
	if b == 0 {
		// simple match
		lr.rep[3], lr.rep[2], lr.rep[1] = lr.rep[2], lr.rep[1], lr.rep[0]
		lr.updateStateMatch()
		// The length decoder returns the length offset.
		n, err := lr.lenCodec.Decode(lr.rd, posState)
		if err != nil {
			return nil, err
		}
		// The dist decoder returns the distance offset. The actual
		// distance is 1 higher.
		lr.rep[0], err = lr.distCodec.Decode(lr.rd, n)
		if err != nil {
			return nil, err
		}
		if lr.rep[0] == eosDist {
			if !lr.rd.possiblyAtEnd() {
				return nil, errWrongTermination
			}
			return nil, eos
		}
		op = match{length: int(n) + minLength,
			distance: int64(lr.rep[0]) + minDistance}
		return op, nil
	}
	b, err = lr.isRepG0[state].Decode(lr.rd)
	if err != nil {
		return nil, err
	}
	dist := lr.rep[0]
	if b == 0 {
		// rep match 0
		b, err = lr.isRepG0Long[state2].Decode(lr.rd)
		if err != nil {
			return nil, err
		}
		if b == 0 {
			lr.updateStateShortRep()
			op = match{length: 1,
				distance: int64(dist) + minDistance}
			return op, nil
		}
	} else {
		b, err = lr.isRepG1[state].Decode(lr.rd)
		if err != nil {
			return nil, err
		}
		if b == 0 {
			dist = lr.rep[1]
		} else {
			b, err = lr.isRepG2[state].Decode(lr.rd)
			if err != nil {
				return nil, err
			}
			if b == 0 {
				dist = lr.rep[2]
			} else {
				dist = lr.rep[3]
				lr.rep[3] = lr.rep[2]
			}
			lr.rep[2] = lr.rep[1]
		}
		lr.rep[1] = lr.rep[0]
		lr.rep[0] = dist
	}
	n, err := lr.repLenCodec.Decode(lr.rd, posState)
	if err != nil {
		return nil, err
	}
	lr.updateStateRep()
	op = match{length: int(n) + minLength,
		distance: int64(dist) + minDistance}
	return op, nil
}

// Indicates that the end of stream marker has been unexpected.
var errUnexpectedEOS = newError("unexpected end-of-stream marker")

// errWrongTermination indicates that a termination symbol has been received,
// but the range decoder could still produces more data
var errWrongTermination = newError("end of stream marker at wrong place")

// fill puts at lest the requested number of bytes into the decoder dictionary.
func (lr *Reader) fill() error {
	if lr.dict.closed {
		return nil
	}
	for lr.dict.Writable() >= maxLength {
		op, err := lr.readOp()
		if err != nil {
			switch {
			case err == eos:
				if lr.params.SizeInHeader &&
					lr.dict.Offset() != lr.params.Size {
					return errUnexpectedEOS
				}
				lr.dict.closed = true
				if !lr.rd.possiblyAtEnd() {
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

		if err = op.applyReaderDict(lr.dict); err != nil {
			return err
		}
		if lr.params.SizeInHeader && lr.dict.Offset() >= lr.params.Size {
			if lr.dict.Offset() > lr.params.Size {
				return newError(
					"more data than announced in header")
			}
			lr.dict.closed = true
			if !lr.rd.possiblyAtEnd() {
				if _, err = lr.readOp(); err != eos {
					return newError(
						"wrong length in header")
				}
				if !lr.rd.possiblyAtEnd() {
					return newError("data after eos")
				}
			}
			return nil
		}
	}
	return nil
}

// Parameters returns the parameters of the LZMA reader. The parameters reflect
// the status provided by the header of the LZMA file.
func (lr *Reader) Parameters() Parameters {
	return *lr.params
}

// Flags for the Reset method of Reader and Writer.
const (
	RState = 1 << iota
	RProperties
	RDict
	RUncompressed
)

// Reset allows the reuse of the LZMA reader using the provide io.Reader. The
// behaviour of the function is controlled by the flags RState, RProperties,
// RDict and RUncompressed.
func (lr *Reader) Reset(r io.Reader, p Properties, flags int) error {
	panic("TODO")
}
