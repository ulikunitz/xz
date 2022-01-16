package lzma

import (
	"bufio"
	"errors"
	"io"
	"math"

	"github.com/ulikunitz/lz"
)

type reader struct {
	dict  lz.Buffer
	state state
	rd    rangeDecoder
	// size < 0 means we wait for EOS
	size int64
	err  error
}

// EOSSize marks a stream that requires the EOS marker to identify the end of
// the stream
const EOSSize uint64 = 1<<64 - 1

// NewRawReader returns a reader that can read a LZMA stream. For a stream with
// an EOS marker use EOSSize for uncompressedSize.
func NewRawReader(z io.Reader, dictSize int, props Properties, uncompressedSize uint64) (r io.Reader, err error) {
	if err = props.Verify(); err != nil {
		return nil, err
	}
	rr := new(reader)
	if err = rr.init(z, dictSize, props, uncompressedSize); err != nil {
		return nil, err
	}
	return rr, nil
}

// minDictSize defines the minumum supported dictionary size.
const minDictSize = 1 << 12

// headerLen defines the length of an LZMA header
const headerLen = 13

// params defines the parameters for the LZMA method
type params struct {
	props            Properties
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
	return h.props.Verify()
}

// append adds the header to the slice s.
func (h params) AppendBinary(p []byte) (r []byte, err error) {
	var a [headerLen]byte
	a[0] = h.props.byte()
	putLE32(a[1:], h.dictSize)
	putLE64(a[5:], h.uncompressedSize)
	return append(p, a[:]...), nil
}

// parse parses the header from the slice x. x must have exactly header length.
func (h *params) UnmarshalBinary(x []byte) error {
	if len(x) != headerLen {
		return errors.New("lzma: LZMA header has incorrect length")
	}
	var err error
	if err = h.props.fromByte(x[0]); err != nil {
		return err
	}
	h.dictSize = getLE32(x[1:])
	h.uncompressedSize = getLE64(x[5:])
	return nil
}

// NewReader creates a new reader for the LZMA streams.
func NewReader(z io.Reader) (r io.Reader, err error) {
	var p = make([]byte, headerLen)
	if _, err = io.ReadFull(z, p); err != nil {
		return nil, err
	}
	var params params
	if err = params.UnmarshalBinary(p); err != nil {
		return nil, err
	}
	if err = params.Verify(); err != nil {
		return nil, err
	}

	if uint64(params.dictSize) > math.MaxInt {
		return nil, errors.New("lzma: dictSize too large")
	}
	d := int(params.dictSize)

	rr := new(reader)
	err = rr.init(z, d, params.props, params.uncompressedSize)
	if err != nil {
		return nil, err
	}

	return rr, nil
}

func (r *reader) init(z io.Reader, dictSize int, props Properties,
	uncompressedSize uint64) error {

	if err := r.dict.Init(dictSize, 2*dictSize); err != nil {
		return err
	}

	r.state.init(props)

	switch {
	case uncompressedSize == EOSSize:
		r.size = -1
	case uncompressedSize <= math.MaxInt64:
		r.size = int64(uncompressedSize)
	default:
		return errors.New("lzma: size overflow")
	}

	br, ok := z.(io.ByteReader)
	if !ok {
		br = bufio.NewReader(z)
	}

	if err := r.rd.init(br); err != nil {
		return err
	}

	switch {
	case uncompressedSize == EOSSize:
		r.size = -1
	case uncompressedSize <= math.MaxInt64:
		r.size = int64(uncompressedSize)
	default:
		return errors.New("lzma: size overflow")
	}

	r.err = nil
	return nil
}

// errEOS informs that an EOS marker has been found
var errEOS = errors.New("EOS marker")

// Distance for EOS marker
const eosDist = 1<<32 - 1

func (r *reader) decodeLiteral() (seq lz.Seq, err error) {
	litState := r.state.litState(r.dict.ByteAtEnd(1), r.dict.Pos())
	match := r.dict.ByteAtEnd(int(r.state.rep[0]) + 1)
	s, err := r.state.litCodec.Decode(&r.rd, r.state.state, match, litState)
	if err != nil {
		return lz.Seq{}, err
	}
	return lz.Seq{LitLen: 1, Aux: uint32(s)}, nil
}

// readSeq reads a single sequence. We are encoding a little bit differently
// than normal, because each seq is either a one-byte literal (LitLen=1, AUX has
// the byte) or a match (MatchLen and Offset non-zero).
func (r *reader) readSeq() (seq lz.Seq, err error) {
	state, state2, posState := r.state.states(r.dict.Pos())

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

// ErrEncoding reports an encoding error
var ErrEncoding = errors.New("lzma: wrong encoding")

func (r *reader) fillBuffer() error {
	if r.err != nil {
		return r.err
	}
	for r.dict.Available() >= maxMatchLen {
		seq, err := r.readSeq()
		if err != nil {
			s := r.size
			switch err {
			case errEOS:
				if r.rd.possiblyAtEnd() && (s < 0 || s == r.dict.Pos()) {
					err = io.EOF
				}
			case io.EOF:
				if !r.rd.possiblyAtEnd() || s != r.dict.Pos() {
					err = io.ErrUnexpectedEOF
				}
			}
			r.err = err
			return err
		}
		if seq.MatchLen == 0 {
			if err = r.dict.WriteByte(byte(seq.Aux)); err != nil {
				panic(err)
			}
		} else {
			err = r.dict.WriteMatch(int(seq.MatchLen),
				int(seq.Offset))
			if err != nil {
				r.err = err
				return err
			}
		}
		if r.size == r.dict.Pos() {
			err = io.EOF
			if !r.rd.possiblyAtEnd() {
				_, serr := r.readSeq()
				if serr != errEOS || !r.rd.possiblyAtEnd() {
					err = ErrEncoding
				}
			}
			r.err = err
			return err
		}
	}
	return nil
}

func (r *reader) Read(p []byte) (n int, err error) {
	for {
		// Read from a dictionary never returns an error
		k, _ := r.dict.Read(p[n:])
		n += k
		if n == len(p) {
			return n, nil
		}
		if err = r.fillBuffer(); err != nil {
			if r.dict.Len() > 0 {
				continue
			}
			return n, err
		}
	}
}
