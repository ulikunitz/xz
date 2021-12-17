package lzma

import (
	"bufio"
	"errors"
	"io"

	"github.com/ulikunitz/lz"
)

// rawReader decompresses a byte stream of LZMA data.
type rawReader struct {
	buf   lz.Buffer
	state state
	rd    rangeDecoder
	eos   bool
}

func (r *rawReader) init(z io.Reader, dictSize int, p Properties) error {
	err := r.buf.Init(dictSize, 2*dictSize)
	if err != nil {
		return err
	}

	// TODO: implement our own byte reader to inline the Read function
	br, ok := z.(io.ByteReader)
	if !ok {
		br = bufio.NewReader(z)
	}
	if err = r.rd.init(br); err != nil {
		return err
	}
	r.state.init(p)

	return nil
}

func (r *rawReader) resetState() {
	panic("TODO")
}

func (r *rawReader) resetProperties(p Properties) error {
	panic("TODO")
}

func (r *rawReader) resetDictionary(p Properties) error {
	panic("TODO")
}

func (r *rawReader) decodeLiteral() (seq lz.Seq, err error) {
	litState := r.state.litState(r.buf.ByteAtEnd(1), r.buf.Pos())
	match := r.buf.ByteAtEnd(int(r.state.rep[0]) + 1)
	s, err := r.state.litCodec.Decode(&r.rd, r.state.state, match, litState)
	if err != nil {
		return lz.Seq{}, err
	}
	return lz.Seq{LitLen: 1, Aux: uint32(s)}, nil
}

var errEOS = errors.New("EOS marker")

// readSeq reads a single sequence. We are encoding a little bit differently
// than normal, because each seq is either a one-byte literal (LitLen=1, AUX has
// the byte) or a match (MatchLen and Offset non-zero).
func (r *rawReader) readSeq() (seq lz.Seq, err error) {
	const eosDist = 1<<32 - 1

	state, state2, posState := r.state.states(r.buf.Pos())

	s2 := &r.state.s2[state2]
	b, err := r.rd.decodeBit(&s2.isMatch)
	if err != nil {
		return lz.Seq{}, err
	}
	if b == 0 {
		// literal
		seq, err := r.decodeLiteral()
		if err != nil {
			return lz.Seq{}, err
		}
		r.state.updateStateLiteral()
		return seq, nil
	}

	s1 := &r.state.s1[state]
	b, err = r.rd.decodeBit(&s1.isRep)
	if err != nil {
		return lz.Seq{}, err
	}
	if b == 0 {
		// simple match
		r.state.rep[3], r.state.rep[2], r.state.rep[1] =
			r.state.rep[2], r.state.rep[1], r.state.rep[0]

		r.state.updateStateMatch()
		// The length decoder returns the length offset.
		n, err := r.state.lenCodec.Decode(&r.rd, posState)
		if err != nil {
			return lz.Seq{}, err
		}
		// The dist decoder returns the distance offset. The actual
		// distance is 1 higher.
		r.state.rep[0], err = r.state.distCodec.Decode(&r.rd, n)
		if err != nil {
			return lz.Seq{}, err
		}
		if r.state.rep[0] == eosDist {
			r.eos = true
			return lz.Seq{}, errEOS
		}
		return lz.Seq{MatchLen: n + minMatchLen,
			Offset: r.state.rep[0] + minDistance}, nil
	}
	b, err = r.rd.decodeBit(&s1.isRepG0)
	if err != nil {
		return lz.Seq{}, err
	}
	dist := r.state.rep[0]
	if b == 0 {
		// rep match 0
		b, err = r.rd.decodeBit(&s2.isRepG0Long)
		if err != nil {
			return lz.Seq{}, err
		}
		if b == 0 {
			r.state.updateStateShortRep()
			return lz.Seq{MatchLen: 1, Offset: dist + minDistance},
				nil
		}
	} else {
		b, err = r.rd.decodeBit(&s1.isRepG1)
		if err != nil {
			return lz.Seq{}, err
		}
		if b == 0 {
			dist = r.state.rep[1]
		} else {
			b, err = r.rd.decodeBit(&s1.isRepG2)
			if err != nil {
				return lz.Seq{}, err
			}
			if b == 0 {
				dist = r.state.rep[2]
			} else {
				dist = r.state.rep[3]
				r.state.rep[3] = r.state.rep[2]
			}
			r.state.rep[2] = r.state.rep[1]
		}
		r.state.rep[1] = r.state.rep[0]
		r.state.rep[0] = dist
	}
	n, err := r.state.repLenCodec.Decode(&r.rd, posState)
	if err != nil {
		return lz.Seq{}, err
	}
	r.state.updateStateRep()
	return lz.Seq{MatchLen: n + minMatchLen, Offset: dist + minDistance},
		nil
}

func (r *rawReader) fillBuffer() error {
	if r.eos {
		return errEOS
	}
	for r.buf.Available() >= maxMatchLen {
		seq, err := r.readSeq()
		if err != nil {
			return err
		}
		if seq.MatchLen == 0 {
			// TODO: remove
			if seq.LitLen != 1 {
				panic("seq has neither literal nor match")
			}
			if err = r.buf.WriteByte(byte(seq.Aux)); err != nil {
				panic(err)
			}
		} else {
			err = r.buf.WriteMatch(int(seq.MatchLen),
				int(seq.Offset))
			if err != nil {
				panic(err)
			}
		}
	}
	return nil
}

func (r *rawReader) Read(p []byte) (n int, err error) {
	if len(p) > r.buf.Len() {
		err = r.fillBuffer()
		if err != nil && err != io.EOF && err != errEOS {
			return 0, err
		}
	}
	n, _ = r.buf.Read(p)
	if n == 0 {
		return 0, err
	}
	return n, nil
}
