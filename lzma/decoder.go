package lzma

import (
	"errors"
	"fmt"
	"io"
)

// Decoder decodes a raw LZMA stream without any header.
type Decoder struct {
	// dictionary; the rear pointer of the buffer will be used for
	// reading the data.
	Dict *DecoderDict
	// decoder state
	State *State
	// range decoder
	rd *rangeDecoder
	// start stores the head value of the dictionary for the LZMA
	// stream
	start int64
	// size of uncompressed data
	size int64
	// eos found
	eos bool
}

// NewDecoder creates a new decoder value. The parameter size provides
// the expected byte size of the decompressed data. If the size is
// unknown use a negative value. In that case the decoder will look for
// a terminating end-of-stream marker.
func NewDecoder(br io.ByteReader, state *State, dict *DecoderDict, size int64) (d *Decoder, err error) {
	rd, err := newRangeDecoder(br)
	if err != nil {
		return nil, err
	}
	d = &Decoder{
		State: state,
		Dict:  dict,
		rd:    rd,
		size:  size,
		start: dict.Pos(),
	}
	return d, nil
}

// Reopens the decoder with a new byte reader and a new size. Reopen
// resets the Decompressed counter to zero.
func (d *Decoder) Reopen(br io.ByteReader, size int64) error {
	var err error
	if d.rd, err = newRangeDecoder(br); err != nil {
		return err
	}
	d.start = d.Dict.Pos()
	d.size = size
	return nil
}

// decodeLiteral decodes a single literal from the LZMA stream.
func (d *Decoder) decodeLiteral() (op operation, err error) {
	litState := d.State.litState(d.Dict.ByteAt(1), d.Dict.head)
	match := d.Dict.ByteAt(int(d.State.rep[0]) + 1)
	s, err := d.State.litCodec.Decode(d.rd, d.State.state, match, litState)
	if err != nil {
		return nil, err
	}
	return lit{s}, nil
}

// errEOS indicates that an EOS marker has been found.
var errEOS = errors.New("EOS marker found")

// readOp decodes the next operation from the compressed stream. It
// returns the operation. If an explicit end of stream marker is
// identified the eos error is returned.
func (d *Decoder) readOp() (op operation, err error) {
	// Value of the end of stream (EOS) marker
	const eosDist = 1<<32 - 1

	state, state2, posState := d.State.states(d.Dict.head)

	b, err := d.State.isMatch[state2].Decode(d.rd)
	if err != nil {
		return nil, err
	}
	if b == 0 {
		// literal
		op, err := d.decodeLiteral()
		if err != nil {
			return nil, err
		}
		d.State.updateStateLiteral()
		return op, nil
	}
	b, err = d.State.isRep[state].Decode(d.rd)
	if err != nil {
		return nil, err
	}
	if b == 0 {
		// simple match
		d.State.rep[3], d.State.rep[2], d.State.rep[1] =
			d.State.rep[2], d.State.rep[1], d.State.rep[0]

		d.State.updateStateMatch()
		// The length decoder returns the length offset.
		n, err := d.State.lenCodec.Decode(d.rd, posState)
		if err != nil {
			return nil, err
		}
		// The dist decoder returns the distance offset. The actual
		// distance is 1 higher.
		d.State.rep[0], err = d.State.distCodec.Decode(d.rd, n)
		if err != nil {
			return nil, err
		}
		if d.State.rep[0] == eosDist {
			return nil, errEOS
		}
		op = match{n: int(n) + minMatchLen,
			distance: int(d.State.rep[0]) + minDistance}
		return op, nil
	}
	b, err = d.State.isRepG0[state].Decode(d.rd)
	if err != nil {
		return nil, err
	}
	dist := d.State.rep[0]
	if b == 0 {
		// rep match 0
		b, err = d.State.isRepG0Long[state2].Decode(d.rd)
		if err != nil {
			return nil, err
		}
		if b == 0 {
			d.State.updateStateShortRep()
			op = match{n: 1, distance: int(dist) + minDistance}
			return op, nil
		}
	} else {
		b, err = d.State.isRepG1[state].Decode(d.rd)
		if err != nil {
			return nil, err
		}
		if b == 0 {
			dist = d.State.rep[1]
		} else {
			b, err = d.State.isRepG2[state].Decode(d.rd)
			if err != nil {
				return nil, err
			}
			if b == 0 {
				dist = d.State.rep[2]
			} else {
				dist = d.State.rep[3]
				d.State.rep[3] = d.State.rep[2]
			}
			d.State.rep[2] = d.State.rep[1]
		}
		d.State.rep[1] = d.State.rep[0]
		d.State.rep[0] = dist
	}
	n, err := d.State.repLenCodec.Decode(d.rd, posState)
	if err != nil {
		return nil, err
	}
	d.State.updateStateRep()
	op = match{n: int(n) + minMatchLen, distance: int(dist) + minDistance}
	return op, nil
}

// apply takes the operation and transforms the decoder dictionary accordingly.
func (d *Decoder) apply(op operation) error {
	switch x := op.(type) {
	case match:
		return d.Dict.WriteMatch(x.distance, x.n)
	case lit:
		return d.Dict.WriteByte(x.b)
	}
	panic("op is neither a match nor a literal")
}

// decompress fills the dictionary unless no space for new data is
// available. If the end of the LZMA stream has been reached io.EOF will
// be returned.
func (d *Decoder) decompress() error {
	if d.eos {
		return io.EOF
	}
	for d.Dict.Available() >= maxMatchLen {
		op, err := d.readOp()
		switch err {
		case nil:
			break
		case errEOS:
			d.eos = true
			if !d.rd.possiblyAtEnd() {
				return errDataAfterEOS
			}
			if d.size >= 0 && d.size != d.Decompressed() {
				return errSize
			}
			return io.EOF
		case io.EOF:
			d.eos = true
			return io.ErrUnexpectedEOF
		default:
			return err
		}
		if err = d.apply(op); err != nil {
			return err
		}
		if d.size >= 0 && d.Decompressed() >= d.size {
			d.eos = true
			if d.Decompressed() > d.size {
				return errSize
			}
			if !d.rd.possiblyAtEnd() {
				switch _, err = d.readOp(); err {
				case nil:
					return errSize
				case io.EOF:
					return io.ErrUnexpectedEOF
				case errEOS:
					break
				default:
					return err
				}
			}
			return io.EOF
		}
	}
	return nil
}

// Errors that may be returned while decoding data.
var (
	errDataAfterEOS = errors.New("lzma: data after end of stream marker")
	errSize         = errors.New("lzma: wrong uncompressed data size")
)

// Read reads data from the buffer. If no more data is available io.EOF is
// returned.
func (d *Decoder) Read(p []byte) (n int, err error) {
	var k int
	for {
		// Read of decoder dict never returns an error.
		k, err = d.Dict.Read(p[n:])
		if err != nil {
			panic(fmt.Errorf("dictionary read error %s", err))
		}
		if k == 0 && d.eos {
			return n, io.EOF
		}
		n += k
		if n >= len(p) {
			return n, nil
		}
		if err = d.decompress(); err != nil && err != io.EOF {
			return n, err
		}
	}
}

// Decompressed returns the number of bytes decompressed by the decoder.
func (d *Decoder) Decompressed() int64 {
	return d.Dict.Pos() - d.start
}
