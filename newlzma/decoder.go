package newlzma

import (
	"errors"
	"fmt"
	"io"
)

type Flags int

const (
	EOSMarker Flags = 1 << iota
	Uncompressed
	NoUncompressedSize
	NoCompressedSize
	ResetState
	ResetProperties
	ResetDict
)

const (
	eos = ResetDict << (iota + 1)
)

type CodecParams struct {
	DictCap          int
	BufCap           int
	CompressedSize   int64
	UncompressedSize int64
	LC               int
	LP               int
	PB               int
	Debug            byte
	Flags            Flags
}

type Decoder struct {
	dict             decoderDict
	state            state
	rd               *rangeDecoder
	start            int64
	uncompressedSize int64
	flags            Flags
	debug            byte
	opCounter        int64
	// reader for uncompressed data
	lr *io.LimitedReader
}

func InitDecoder(d *Decoder, r io.Reader, p CodecParams) error {
	*d = Decoder{}
	if err := initDecoderDict(&d.dict, p.DictCap, p.BufCap); err != nil {
		return err
	}
	p.Flags |= ResetDict
	d.Reset(r, p)
	return nil
}

func NewDecoder(r io.Reader, p CodecParams) (d *Decoder, err error) {
	d = new(Decoder)
	if err = InitDecoder(d, r, p); err != nil {
		return nil, err
	}
	return d, nil
}

func (d *Decoder) Reset(r io.Reader, p CodecParams) error {
	d.flags = p.Flags
	d.uncompressedSize = p.UncompressedSize
	d.debug = p.Debug

	if p.Flags&ResetDict != 0 {
		d.dict.Reset()
	}
	d.start = d.dict.head

	if d.flags&Uncompressed != 0 {
		if d.flags&NoUncompressedSize != 0 {
			panic("uncompressed segment needs size")
		}
		d.lr = &io.LimitedReader{R: r, N: d.uncompressedSize}
		return nil
	}

	if p.Flags&(ResetProperties|ResetDict) != 0 {
		props, err := NewProperties(p.LC, p.LP, p.PB)
		if err != nil {
			return err
		}
		initState(&d.state, props)
	} else if p.Flags&ResetState != 0 {
		d.state.Reset()
	}

	var err error
	if p.Flags&NoCompressedSize != 0 {
		d.rd, err = newRangeDecoder(r)
	} else {
		d.rd, err = newRangeDecoderLimit(r, p.CompressedSize)
	}
	return err
}

func (d *Decoder) decodeLiteral() (op operation, err error) {
	litState := d.state.litState(d.dict.ByteAt(1), d.dict.head)
	match := d.dict.ByteAt(int(d.state.rep[0]) + 1)
	s, err := d.state.litCodec.Decode(d.rd, d.state.state, match, litState)
	if err != nil {
		return nil, err
	}
	return lit{s}, nil
}

func (d *Decoder) verifyEOS() error {
	if d.flags&NoCompressedSize == 0 && d.rd.r.limit != d.rd.r.n {
		return ErrCompressedSizeWrong
	}
	if d.flags&NoUncompressedSize == 0 && d.uncompressedSize != d.Uncompressed() {
		return ErrUncompressedSizeWrong
	}
	return nil
}

func (d *Decoder) handleEOSMarker() error {
	d.flags |= eos
	if !d.rd.possiblyAtEnd() {
		return ErrMoreData
	}
	return d.verifyEOS()
}

func (d *Decoder) handleEOS() error {
	d.flags |= eos
	if d.flags&EOSMarker != 0 {
		_, err := d.readOp()
		if err != nil {
			if err == errEOS {
				return nil
			}
			return err
		}
		return ErrMissingEOSMarker
	}
	if !d.rd.possiblyAtEnd() ||
		(d.flags&NoCompressedSize == 0 && d.rd.r.n < d.rd.r.limit) {

		op, err := d.readOp()
		if err != nil {
			if err == errEOS {
				return nil
			}
			return err
		}
		if err = d.apply(op); err != nil {
			return err
		}
		if err = d.verifyEOS(); err != nil {
			return err
		}
		panic("read one more op without an error")
	}
	return d.verifyEOS()
}

var errEOS = errors.New("EOS marker found")

// readOp decodes the next operation from the compressed stream. It
// returns the operation. If an explicit end of stream marker is
// identified the eos error is returned.
func (d *Decoder) readOp() (op operation, err error) {
	// Value of the end of stream (EOS) marker
	const eosDist = 1<<32 - 1

	state, state2, posState := d.state.states(d.dict.head)

	b, err := d.state.isMatch[state2].Decode(d.rd)
	if err != nil {
		return nil, err
	}
	if b == 0 {
		// literal
		op, err := d.decodeLiteral()
		if err != nil {
			return nil, err
		}
		d.state.updateStateLiteral()
		return op, nil
	}
	b, err = d.state.isRep[state].Decode(d.rd)
	if err != nil {
		return nil, err
	}
	if b == 0 {
		// simple match
		d.state.rep[3], d.state.rep[2], d.state.rep[1] =
			d.state.rep[2], d.state.rep[1], d.state.rep[0]

		d.state.updateStateMatch()
		// The length decoder returns the length offset.
		n, err := d.state.lenCodec.Decode(d.rd, posState)
		if err != nil {
			return nil, err
		}
		// The dist decoder returns the distance offset. The actual
		// distance is 1 higher.
		d.state.rep[0], err = d.state.distCodec.Decode(d.rd, n)
		if err != nil {
			return nil, err
		}
		if d.state.rep[0] == eosDist {
			if err = d.handleEOSMarker(); err == nil {
				err = errEOS
			}
			return nil, err
		}
		op = match{n: int(n) + MinMatchLen,
			distance: int(d.state.rep[0]) + minDistance}
		return op, nil
	}
	b, err = d.state.isRepG0[state].Decode(d.rd)
	if err != nil {
		return nil, err
	}
	dist := d.state.rep[0]
	if b == 0 {
		// rep match 0
		b, err = d.state.isRepG0Long[state2].Decode(d.rd)
		if err != nil {
			return nil, err
		}
		if b == 0 {
			d.state.updateStateShortRep()
			op = match{n: 1, distance: int(dist) + minDistance}
			return op, nil
		}
	} else {
		b, err = d.state.isRepG1[state].Decode(d.rd)
		if err != nil {
			return nil, err
		}
		if b == 0 {
			dist = d.state.rep[1]
		} else {
			b, err = d.state.isRepG2[state].Decode(d.rd)
			if err != nil {
				return nil, err
			}
			if b == 0 {
				dist = d.state.rep[2]
			} else {
				dist = d.state.rep[3]
				d.state.rep[3] = d.state.rep[2]
			}
			d.state.rep[2] = d.state.rep[1]
		}
		d.state.rep[1] = d.state.rep[0]
		d.state.rep[0] = dist
	}
	n, err := d.state.repLenCodec.Decode(d.rd, posState)
	if err != nil {
		return nil, err
	}
	d.state.updateStateRep()
	op = match{n: int(n) + MinMatchLen, distance: int(dist) + minDistance}
	return op, nil
}

func (d *Decoder) Printf(format string, args ...interface{}) (n int, err error) {
	if d.debug != 0 {
		return fmt.Printf(format, args...)
	}
	return 0, nil
}

func (d *Decoder) apply(op operation) error {
	d.opCounter++
	d.Printf("%d %s\n", d.opCounter, op)
	switch x := op.(type) {
	case match:
		return d.dict.WriteMatch(x.distance, x.n)
	case lit:
		return d.dict.WriteByte(x.b)
	}
	panic("op is neither a match nor a literal")
}

func (d *Decoder) fillDict() error {
	if d.flags&eos != 0 {
		return nil
	}
	for d.dict.Available() >= MaxMatchLen {
		op, err := d.readOp()
		switch err {
		case nil:
			break
		case errEOS:
			return nil
		case io.EOF:
			d.flags |= eos
			return io.ErrUnexpectedEOF
		default:
			return err
		}
		if err = d.apply(op); err != nil {
			return err
		}
		if d.flags&NoUncompressedSize == 0 && d.Uncompressed() >=
			d.uncompressedSize {

			return d.handleEOS()
		}
		if d.flags&NoCompressedSize == 0 && d.rd.r.n >= d.rd.r.limit {
			return d.handleEOS()
		}
	}
	return nil
}

func (d *Decoder) fillDictUncompressed() error {
	if d.flags&Uncompressed == 0 {
		panic("Uncompressed flag not set")
	}
	if d.flags&eos != 0 {
		return nil
	}
	for d.dict.Available() >= 0 {
		_, err := io.CopyN(&d.dict, d.lr, int64(d.dict.Available()))
		if err != nil {
			if err == io.EOF {
				d.flags |= eos
				if d.lr.N != 0 {
					return ErrUncompressedSizeWrong
				}
				return nil
			}
		}
	}
	return nil
}

var (
	ErrMissingEOSMarker      = errors.New("EOS marker is missing")
	ErrMoreData              = errors.New("more data after EOS")
	ErrCompressedSizeWrong   = errors.New("compressed size wrong")
	ErrUncompressedSizeWrong = errors.New("uncompressed size wrong")
)

func (d *Decoder) Read(p []byte) (n int, err error) {
	fillDict := d.fillDict
	if d.flags&Uncompressed != 0 {
		fillDict = d.fillDictUncompressed
	}

	var k int
	for n < len(p) {
		if err = fillDict(); err != nil {
			return
		}
		// Read of decoder dict never returns an error.
		k, err = d.dict.Read(p[n:])
		if err != nil {
			panic(fmt.Errorf("dictionary read error %s", err))
		}
		if k == 0 && d.flags&eos != 0 {
			return n, io.EOF
		}
		n += k
	}
	return
}

func (d *Decoder) Compressed() int64 {
	return d.rd.Compressed()
}

func (d *Decoder) Uncompressed() int64 {
	return d.dict.head - d.start
}
