package lzma

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"math"

	"github.com/ulikunitz/lz"
)

// Properties define the properties for the LZMA and LZMA2 compression.
type Properties struct {
	LC int
	LP int
	PB int
}

// Returns the byte that encodes the properties.
func (p Properties) byte() byte {
	return (byte)((p.PB*5+p.LP)*9 + p.LC)
}

func (p *Properties) fromByte(b byte) error {
	p.LC = int(b % 9)
	b /= 9
	p.LP = int(b % 5)
	b /= 5
	p.PB = int(b)
	if p.PB > 4 {
		return errors.New("lzma: invalid properties byte")
	}
	return nil
}

func (p Properties) Verify() error {
	if !(0 <= p.LC && p.LC <= 8) {
		return fmt.Errorf("lzma: LC out of range 0..8")
	}
	if !(0 <= p.LP && p.LP <= 4) {
		return fmt.Errorf("lzma: LP out of range 0..4")
	}
	if !(0 <= p.PB && p.PB <= 4) {
		return fmt.Errorf("lzma: PB out of range 0..4")
	}
	return nil
}

// eosSize is used for the uncompressed size if it is unknown
const eosSize uint64 = 0xffffffffffffffff

// headerLen defines the length of an LZMA header
const headerLen = 13

// params defines the parameters for the LZMA method
type params struct {
	p                Properties
	dictSize         uint32
	uncompressedSize uint64
}

func (h params) Verify() error {
	if uint64(h.dictSize) > math.MaxInt {
		return errors.New("lzma: dictSize exceed max integer")
	}
	if h.dictSize < minDictSize {
		return errors.New("lzma: dictSize is too small")
	}
	return h.p.Verify()
}

// append adds the header to the slice s.
func (h params) append(s []byte) []byte {
	var a [headerLen]byte
	a[0] = h.p.byte()
	putLE32(a[1:], h.dictSize)
	putLE64(a[5:], h.uncompressedSize)
	return append(s, a[:]...)
}

// parse parses the header from the slice x. x must have exactly header length.
func (h *params) parse(x []byte) error {
	if len(x) != headerLen {
		return errors.New("lzma: LZMA header has incorrect length")
	}
	var err error
	if err = h.p.fromByte(x[0]); err != nil {
		return err
	}
	h.dictSize = getLE32(x[1:])
	h.uncompressedSize = getLE64(x[5:])
	return nil
}

// rawReader decompresses a byte stream of LZMA data.
type rawReader struct {
	buf   lz.Buffer
	state state
	rd    rangeDecoder
	p     params
	err   error
}

func (r *rawReader) init(z io.Reader, p params) error {
	r.p = p

	err := r.buf.Init(int(p.dictSize), 2*int(p.dictSize))
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
	r.state.init(p.p)

	return nil
}

func (r *rawReader) restart() {
	panic("TODO")
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
	if r.err != nil {
		return r.err
	}
	for r.buf.Available() >= maxMatchLen {
		seq, err := r.readSeq()
		if err != nil {
			if err == errEOS {
				if !r.rd.possiblyAtEnd() {
					r.err = ErrUnexpectedEOS
					return r.err
				}
				s := r.p.uncompressedSize
				if s != eosSize && s != uint64(r.buf.Pos()) {
					r.err = ErrUnexpectedEOS
					return r.err
				}
				r.err = io.EOF
				return r.err
			}
			if err == io.EOF {
				s := r.p.uncompressedSize
				if !r.rd.possiblyAtEnd() || s == eosSize {
					r.err = io.ErrUnexpectedEOF
					return r.err
				}
				if s != uint64(r.buf.Pos()) {
					r.err = io.ErrUnexpectedEOF
				}
				r.err = io.EOF
				return r.err
			}
			r.err = err
			return r.err
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
		s := r.p.uncompressedSize
		if s == uint64(r.buf.Pos()) {
			r.err = io.EOF
			return r.err
		}
	}
	return nil
}

func (r *rawReader) Read(p []byte) (n int, err error) {
	if len(p) > r.buf.Len() {
		err = r.fillBuffer()
		if err != nil && err != io.EOF {
			return 0, err
		}
	}
	n, _ = r.buf.Read(p)
	if n == 0 {
		return 0, err
	}
	return n, nil
}
