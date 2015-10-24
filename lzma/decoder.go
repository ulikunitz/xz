package lzma

import (
	"errors"
	"fmt"
	"io"
)

// The CodecParams provides the parameters for the encoder and decoder.
// Note that the size fields are limits for encoding and fixed values
// for decoding.
//
// The debug flag is currently only supported by the LZMA decoder. The
// stream of operations is printed if the debug field is set to any
// value not equal to zero.
type CodecParams struct {
	// uncompressed size; negative if no size is provided
	Size int64
	// Debug defines the debug level
	Debug byte
	// true if EOS Marker is expected or should be written
	EOSMarker bool
}

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
	// size stores the value of the CodecParams value
	size int64
	// eosMarker flag
	eosMarker bool
	// eos found
	eos bool
	// debug value as provided by the CodecParams value
	debug byte
	// opCounter counts operations
	opCounter int64
}

// Init initializes the decoder structure.
func (d *Decoder) Init(br io.ByteReader, state *State, dict *DecoderDict,
	p CodecParams) error {
	*d = Decoder{}
	d.State = state
	d.Dict = dict
	var err error
	if d.rd, err = newRangeDecoder(br); err != nil {
		return err
	}
	d.size = p.Size
	d.eosMarker = p.EOSMarker
	if d.size < 0 {
		d.eosMarker = true
	}
	d.debug = p.Debug
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

// verifyEOS checks whether the decoder is indeed at the end of the LZMA
// stream.
func (d *Decoder) verifyEOS() error {
	if d.size >= 0 && d.size != d.Uncompressed() {
		return errUncompressedSize
	}
	return nil
}

// handleEOSMarker handles an identified ESO marker.
func (d *Decoder) handleEOSMarker() error {
	d.eos = true
	if !d.rd.possiblyAtEnd() {
		return errMoreData
	}
	return d.verifyEOS()
}

// handleEOS handles an identified EOS condition, but not the
// identification of an EOS marker.
func (d *Decoder) handleEOS() error {
	d.eos = true
	if d.eosMarker {
		_, err := d.readOp()
		if err != nil {
			if err == errEOS {
				return nil
			}
			return err
		}
		return errMissingEOSMarker
	}
	if !d.rd.possiblyAtEnd() {
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
			if err = d.handleEOSMarker(); err == nil {
				err = errEOS
			}
			return nil, err
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

// Printf produces output if d.debug is not zero. It uses fmt.Printf
// for the implementation.
func (d *Decoder) Printf(format string, args ...interface{}) (n int, err error) {
	if d.debug != 0 {
		return fmt.Printf(format, args...)
	}
	return 0, nil
}

// apply takes the operation and transforms the decoder dictionary accordingly.
func (d *Decoder) apply(op operation) error {
	d.opCounter++
	d.Printf("%d %s\n", d.opCounter, op)
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
			return d.handleEOS()
		}
	}
	return nil
}

// Errors that may be returned while decoding data.
var (
	errMissingEOSMarker = errors.New("EOS marker is missing")
	errMoreData         = errors.New("more data after EOS")
	errCompressedSize   = errors.New("compressed size wrong")
	errUncompressedSize = errors.New("uncompressed size wrong")
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
