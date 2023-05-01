package lzma

import (
	"bufio"
	"errors"
	"io"
	"math"

	"github.com/ulikunitz/lz"
)

// reader supports the reading of an LZMA stream.
type reader struct {
	decoder
	// size < 0 means we wait for EOS
	size int64
	err  error
}

// EOSSize marks a stream that requires the EOS marker to identify the end of
// the stream. It is used by [NewRawReader].
const EOSSize uint64 = 1<<64 - 1

// NewRawReader returns a reader that can read a LZMA stream. For a stream with
// an EOS marker use [EOSSize] for uncompressedSize. The dictSize must be
// positive (>=0).
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

// minDictSize defines the minimum supported dictionary size.
const minDictSize = 1 << 12

// headerLen defines the length of an LZMA header
const headerLen = 13

// params defines the parameters for the LZMA method
type params struct {
	props            Properties
	dictSize         uint32
	uncompressedSize uint64
}

// Verify checks the parameters for correctness.
func (h params) Verify() error {
	if uint64(h.dictSize) > math.MaxInt {
		return errors.New("lzma: dictSize exceed max integer")
	}
	if h.dictSize < minDictSize {
		return errors.New("lzma: dictSize is too small")
	}
	return h.props.Verify()
}

// AppendBinary adds the header to the slice s.
func (h params) AppendBinary(p []byte) (r []byte, err error) {
	var a [headerLen]byte
	a[0] = h.props.byte()
	putLE32(a[1:], h.dictSize)
	putLE64(a[5:], h.uncompressedSize)
	return append(p, a[:]...), nil
}

// UnmarshalBinary parses the header from the slice x. x must have exactly header length.
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
	// The LZMA specification says that if the dictionary size in the header
	// is less than 4096 it must be set to 4096. See pull request
	// https://github.com/ulikunitz/xz/pull/52
	// TODO: depending on the discussion we might even need a way to
	// override the header.
	if params.dictSize < minDictSize {
		params.dictSize = minDictSize
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

// init initializes the reader.
func (r *reader) init(z io.Reader, dictSize int, props Properties,
	uncompressedSize uint64) error {

	if err := r.buffer.Init(lz.DecoderConfig{WindowSize: dictSize}); err != nil {
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

// ErrEncoding reports an encoding error
var ErrEncoding = errors.New("lzma: wrong encoding")

// fillBuffer refills the buffer.
func (r *reader) fillBuffer() error {
	for {
		if a := r.buffer.BufferSize - len(r.buffer.Data); a < maxMatchLen {
			break
		}
		seq, err := r.readSeq()
		if err != nil {
			s := r.size
			switch err {
			case errEOS:
				if r.rd.possiblyAtEnd() && (s < 0 || s == r.buffer.Off) {
					err = io.EOF
				}
			case io.EOF:
				if !r.rd.possiblyAtEnd() || s != r.buffer.Off {
					err = io.ErrUnexpectedEOF
				}
			}
			return err
		}
		if seq.MatchLen == 0 {
			if err = r.buffer.WriteByte(byte(seq.Aux)); err != nil {
				panic(err)
			}
		} else {
			_, err = r.buffer.WriteMatch(seq.MatchLen, seq.Offset)
			if err != nil {
				return err
			}
		}
		if r.size == r.buffer.Off {
			err = io.EOF
			if !r.rd.possiblyAtEnd() {
				_, serr := r.readSeq()
				if serr != errEOS || !r.rd.possiblyAtEnd() {
					err = ErrEncoding
				}
			}
			return err
		}
	}
	return nil
}

// Read reads data from the dictionary and refills it if needed.
func (r *reader) Read(p []byte) (n int, err error) {
	k := len(r.buffer.Data) - r.buffer.R
	if r.err != nil && k == 0 {
		return 0, r.err
	}
	for {
		// Read from a dictionary never returns an error
		k, _ := r.buffer.Read(p[n:])
		n += k
		if n == len(p) {
			return n, nil
		}
		if r.err != nil {
			return n, r.err
		}
		if err = r.fillBuffer(); err != nil {
			r.err = err
			k := len(r.buffer.Data) - r.buffer.R
			if k > 0 {
				continue
			}
			return n, err
		}
	}
}
