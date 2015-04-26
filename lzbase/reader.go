package lzbase

import "io"

// Reader supports the reading of raw LZMA streams without any header
// information. So it can be used by LZMA and LZMA2 streams.
type Reader struct {
	State *ReaderState
	rd    *rangeDecoder
	dict  *ReaderDict
}

// NewReader creates a raw LZMA stream by using the reader state. It should be
// noted that the ReaderState contains the Dictionary.
func NewReader(r io.Reader, state *ReaderState) (*Reader, error) {
	rd, err := newRangeDecoder(r)
	if err != nil {
		return nil, err
	}
	return &Reader{State: state, rd: rd, dict: state.ReaderDict()}, nil
}

// Reads reads data from the decoder stream.
//
// The end of the LZMA stream is indicated by EOF. There might be other errors
// returned. The decoder will not be able to recover from an error returned.
func (br *Reader) Read(p []byte) (n int, err error) {
	for {
		var k int
		k, err = br.dict.read(p[n:])
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
func (br *Reader) decodeLiteral() (op Operation, err error) {
	litState := br.State.litState()

	match := br.dict.byteAt(int64(br.State.rep[0]) + 1)
	s, err := br.State.litCodec.Decode(br.rd, br.State.state.state, match,
		litState)
	if err != nil {
		return nil, err
	}
	return lit{s}, nil
}

// errWrongTermination indicates that a termination symbol has been received,
// but the range decoder could still produces more data
var errWrongTermination = newError("end of stream marker at wrong place")

// eos indicates an explicit end of stream
var eos = newError("end of decoded stream")

// readOp decodes the next operation from the compressed stream. It returns the
// operation. If an exlicit end of stream marker is identified the eos error is
// returned.
func (br *Reader) readOp() (op Operation, err error) {
	state, state2, posState := br.State.states()

	b, err := br.State.isMatch[state2].Decode(br.rd)
	if err != nil {
		return nil, err
	}
	if b == 0 {
		// literal
		op, err := br.decodeLiteral()
		if err != nil {
			return nil, err
		}
		br.State.updateStateLiteral()
		return op, nil
	}
	b, err = br.State.isRep[state].Decode(br.rd)
	if err != nil {
		return nil, err
	}
	if b == 0 {
		// simple match
		br.State.rep[3], br.State.rep[2], br.State.rep[1] = br.State.rep[2], br.State.rep[1], br.State.rep[0]

		br.State.updateStateMatch()
		// The length decoder returns the length offset.
		n, err := br.State.lenCodec.Decode(br.rd, posState)
		if err != nil {
			return nil, err
		}
		// The dist decoder returns the distance offset. The actual
		// distance is 1 higher.
		br.State.rep[0], err = br.State.distCodec.Decode(br.rd, n)
		if err != nil {
			return nil, err
		}
		if br.State.rep[0] == eosDist {
			if !br.rd.possiblyAtEnd() {
				return nil, errWrongTermination
			}
			return nil, eos
		}
		op = match{n: int(n) + MinLength,
			distance: int64(br.State.rep[0]) + minDistance}
		return op, nil
	}
	b, err = br.State.isRepG0[state].Decode(br.rd)
	if err != nil {
		return nil, err
	}
	dist := br.State.rep[0]
	if b == 0 {
		// rep match 0
		b, err = br.State.isRepG0Long[state2].Decode(br.rd)
		if err != nil {
			return nil, err
		}
		if b == 0 {
			br.State.updateStateShortRep()
			op = match{n: 1, distance: int64(dist) + minDistance}
			return op, nil
		}
	} else {
		b, err = br.State.isRepG1[state].Decode(br.rd)
		if err != nil {
			return nil, err
		}
		if b == 0 {
			dist = br.State.rep[1]
		} else {
			b, err = br.State.isRepG2[state].Decode(br.rd)
			if err != nil {
				return nil, err
			}
			if b == 0 {
				dist = br.State.rep[2]
			} else {
				dist = br.State.rep[3]
				br.State.rep[3] = br.State.rep[2]
			}
			br.State.rep[2] = br.State.rep[1]
		}
		br.State.rep[1] = br.State.rep[0]
		br.State.rep[0] = dist
	}
	n, err := br.State.repLenCodec.Decode(br.rd, posState)
	if err != nil {
		return nil, err
	}
	br.State.updateStateRep()
	op = match{n: int(n) + MinLength, distance: int64(dist) + minDistance}
	return op, nil
}

// fill reads operations and fills the dictionary.
func (br *Reader) fill() error {
	if br.dict.closed {
		return nil
	}
	for br.dict.writable() >= MaxLength {
		op, err := br.readOp()
		if err != nil {
			switch {
			case err == eos:
				br.dict.closed = true
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

		if err = op.Apply(br.dict); err != nil {
			return err
		}
	}
	return nil
}
