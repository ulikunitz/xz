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

// Init initializes the decoder structure. The parameter size must be
// negative if no size is given. In such a case an EOS marker is
// expected.
func (d *Decoder) Init(br io.ByteReader, state *State, dict *DecoderDict,
	size int64) error {

	*d = Decoder{}
	d.State = state
	d.Dict = dict
	var err error
	if d.rd, err = newRangeDecoder(br); err != nil {
		return err
	}
	d.size = size
	d.start = d.Dict.Pos()
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

// fillDict fills the dictionary unless no space for new data is
// available in the dictionary.
func (d *Decoder) fillDict() error {
	if d.eos {
		return nil
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
			if d.size >= 0 && d.size != d.Uncompressed() {
				return errSize
			}
			return nil
		case io.EOF:
			d.eos = true
			return io.ErrUnexpectedEOF
		default:
			return err
		}
		if err = d.apply(op); err != nil {
			return err
		}
		if d.size >= 0 && d.Uncompressed() >= d.size {
			d.eos = true
			if d.Uncompressed() > d.size {
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
			return nil
		}
	}
	return nil
}

// Errors that may be returned while decoding data.
var (
	errDataAfterEOS = errors.New("lzma: data after end of stream marker")
	errSize         = errors.New("lzma: wrong uncompressed data size")
)

// Read reads data from the buffer. If no more data is available EOF is
// returned.
func (d *Decoder) Read(p []byte) (n int, err error) {
	var k int
	for n < len(p) {
		if err = d.fillDict(); err != nil {
			return
		}
		// Read of decoder dict never returns an error.
		k, err = d.Dict.Read(p[n:])
		if err != nil {
			panic(fmt.Errorf("dictionary read error %s", err))
		}
		if k == 0 && d.eos {
			return n, io.EOF
		}
		n += k
	}
	return
}

// Uncompressed returns the number of uncompressed bytes decoded.
func (d *Decoder) Uncompressed() int64 {
	return d.Dict.Pos() - d.start
}
