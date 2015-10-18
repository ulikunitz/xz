package lzma

import (
	"errors"
	"fmt"
	"io"
)

// The CFlags type is used in the CodecParams structure to store flags.
type CFlags int

// Flags for the CodecParams structure.
const (
	// EOS marker must be written or will be present.
	CEOSMarker CFlags = 1 << iota
	// The data will be or is not compressed.
	CUncompressed
	// No uncompressed size is provided.
	CNoUncompressedSize
	// No compressed size is provided.
	CNoCompressedSize
	// If the CodecParams value is used with Reset describes which
	// part of the encoder or decoder must be reset.
	CResetState
	CResetProperties
	CResetDict
)

// Flag indicates in a decoder that the end of stream has been reached.
const ceos = CResetDict << (iota + 1)

// The CodecParams provides the parameters for the encoder and decoder.
// Note that the size fields are limits for encoding and fixed values
// for decoding.
//
// The debug flag is currently only supported by the LZMA decoder. The
// stream of operations is printed if the debug field is set to any
// value not equal to zero.
type CodecParams struct {
	// dictionary capacity
	DictCap int
	// buffer capacity
	BufCap int
	// compressed size; see CNoCompressedSize
	CompressedSize int64
	// uncompressed size; see CNoUncompressedSize
	UncompressedSize int64
	// literal context
	LC int
	// literal position bits
	LP int
	// position bits
	PB int
	// Debug defines the debug level
	Debug byte
	// boolean flags are stored here
	Flags CFlags
}

// Decoder decodes a raw LZMA stream without any header.
type Decoder struct {
	// dictionary; the rear pointer of the buffer will be used for
	// reading the data.
	dict decoderDict
	// decoder state
	state State
	// range decoder
	rd *rangeDecoder
	// start stores the head value of the dictionary for the LZMA
	// stream
	start int64
	// uncompressedSize stors the value of the CodecParams  value
	uncompressedSize int64
	// flags as provided by the CodecParams value
	flags CFlags
	// debug value as provided by the CodecParams value
	debug byte
	// counter of operations used for debug output
	opCounter int64
	// reader for uncompressed data
	lr *io.LimitedReader
}

// InitDecoder initializes a LZMA decoder value.
func InitDecoder(d *Decoder, r io.Reader, p *CodecParams) error {
	*d = Decoder{}
	if err := initDecoderDict(&d.dict, p.DictCap, p.BufCap); err != nil {
		return err
	}
	p.Flags |= CResetDict
	err := d.Reset(r, p)
	return err
}

// NewDecoder allocates and initializes a new LZMA decoder.
func NewDecoder(r io.Reader, p *CodecParams) (d *Decoder, err error) {
	d = new(Decoder)
	if err = InitDecoder(d, r, p); err != nil {
		return nil, err
	}
	return d, nil
}

// Reset resets the decoder. Note that the buffer will contain its
// current value.
func (d *Decoder) Reset(r io.Reader, p *CodecParams) error {
	d.flags = p.Flags
	d.uncompressedSize = p.UncompressedSize
	d.debug = p.Debug

	if p.Flags&CResetDict != 0 {
		d.dict.Reset()
	}
	d.start = d.dict.head

	if d.flags&CUncompressed != 0 {
		if d.flags&CNoUncompressedSize != 0 {
			panic("uncompressed segment needs size")
		}
		d.lr = &io.LimitedReader{R: r, N: d.uncompressedSize}
		return nil
	}

	if p.Flags&(CResetProperties|CResetDict) != 0 {
		props, err := NewProperties(p.LC, p.LP, p.PB)
		if err != nil {
			return err
		}
		initState(&d.state, props)
	} else if p.Flags&CResetState != 0 {
		d.state.Reset()
	}

	var err error
	if p.Flags&CNoCompressedSize != 0 {
		d.rd, err = newRangeDecoder(r)
	} else {
		d.rd, err = newRangeDecoderLimit(r, p.CompressedSize)
	}
	return err
}

// decodeLiteral decodes a single literal from the LZMA stream.
func (d *Decoder) decodeLiteral() (op operation, err error) {
	litState := d.state.litState(d.dict.ByteAt(1), d.dict.head)
	match := d.dict.ByteAt(int(d.state.rep[0]) + 1)
	s, err := d.state.litCodec.Decode(d.rd, d.state.state, match, litState)
	if err != nil {
		return nil, err
	}
	return lit{s}, nil
}

// verifyEOS checks whether the decoder is indeed at the end of the LZMA
// stream.
func (d *Decoder) verifyEOS() error {
	if d.flags&CNoCompressedSize == 0 && d.rd.r.limit != d.rd.r.n {
		return errCompressedSize
	}
	if d.flags&CNoUncompressedSize == 0 && d.uncompressedSize != d.Uncompressed() {
		return errUncompressedSize
	}
	return nil
}

// handleEOSMarker handles an identified ESO marker.
func (d *Decoder) handleEOSMarker() error {
	d.flags |= ceos
	if !d.rd.possiblyAtEnd() {
		return errMoreData
	}
	return d.verifyEOS()
}

// handleEOS handles an identified EOS condition, but not the
// identification of an EOS marker.
func (d *Decoder) handleEOS() error {
	d.flags |= ceos
	if d.flags&CEOSMarker != 0 {
		_, err := d.readOp()
		if err != nil {
			if err == errEOS {
				return nil
			}
			return err
		}
		return errMissingEOSMarker
	}
	if !d.rd.possiblyAtEnd() ||
		(d.flags&CNoCompressedSize == 0 && d.rd.r.n < d.rd.r.limit) {

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
		op = match{n: int(n) + minMatchLen,
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
		return d.dict.WriteMatch(x.distance, x.n)
	case lit:
		return d.dict.WriteByte(x.b)
	}
	panic("op is neither a match nor a literal")
}

// fillDict fills the dictionary unless no space for new data is
// available in the dictionary.
func (d *Decoder) fillDict() error {
	if d.flags&ceos != 0 {
		return nil
	}
	for d.dict.Available() >= maxMatchLen {
		op, err := d.readOp()
		switch err {
		case nil:
			break
		case errEOS:
			return nil
		case io.EOF:
			d.flags |= ceos
			return io.ErrUnexpectedEOF
		default:
			return err
		}
		if err = d.apply(op); err != nil {
			return err
		}
		if d.flags&CNoUncompressedSize == 0 && d.Uncompressed() >=
			d.uncompressedSize {

			return d.handleEOS()
		}
		if d.flags&CNoCompressedSize == 0 && d.rd.r.n >= d.rd.r.limit {
			return d.handleEOS()
		}
	}
	return nil
}

// fillDictUncompressed fills the dictionary if the input stream isn't
// compressed.
func (d *Decoder) fillDictUncompressed() error {
	if d.flags&CUncompressed == 0 {
		panic("Uncompressed flag not set")
	}
	if d.flags&ceos != 0 {
		return nil
	}
	for d.dict.Available() >= 0 {
		_, err := io.CopyN(&d.dict, d.lr, int64(d.dict.Available()))
		if err != nil {
			if err == io.EOF {
				d.flags |= ceos
				if d.lr.N != 0 {
					return errUncompressedSize
				}
				return nil
			}
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
	fillDict := d.fillDict
	if d.flags&CUncompressed != 0 {
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
		if k == 0 && d.flags&ceos != 0 {
			return n, io.EOF
		}
		n += k
	}
	return
}

// Compressed returns the number of compressed bytes read.
func (d *Decoder) Compressed() int64 {
	return d.rd.Compressed()
}

// Uncompressed returns the number of uncompressed bytes decoded.
func (d *Decoder) Uncompressed() int64 {
	return d.dict.head - d.start
}
