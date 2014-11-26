package lzma

import (
	"bufio"
	"io"

	"github.com/uli-go/xz/xlog"
)

// states defines the overall state count
const states = 12

// bufferLen is the value used for the bufferLen used by the decoder.
var bufferLen = 64 * (1 << 10)

// noUnpackLen requires an explicit end of stream marker
const noUnpackLen uint64 = 1<<64 - 1

// Reader is able to read a LZMA byte stream and to read the plain text.
//
// Note that an unpackLen of 0xffffffffffffffff requires an explicit end of
// stream marker.
type Reader struct {
	properties Properties
	// length to unpack
	unpackLen        uint64
	decodedLen       uint64
	dict             *decoderDict
	state            uint32
	posBitMask       uint32
	rd               *rangeDecoder
	isMatch          [states << maxPosBits]prob
	isRep            [states]prob
	isRepG0          [states]prob
	isRepG1          [states]prob
	isRepG2          [states]prob
	isRepG0Long      [states << maxPosBits]prob
	rep              [4]uint32
	litDecoder       *literalCodec
	lengthDecoder    *lengthCodec
	repLengthDecoder *lengthCodec
	distDecoder      *distCodec
}

// NewReader creates an LZMA reader. It reads the classic, original LZMA
// format. Note that LZMA2 uses a different header format.
func NewReader(r io.Reader) (*Reader, error) {
	f := bufio.NewReader(r)
	properties, err := readProperties(f)
	if err != nil {
		return nil, err
	}
	historyLen := int(properties.DictLen)
	if historyLen < 0 {
		return nil, newError(
			"LZMA property DictLen exceeds maximum int value")
	}
	l := &Reader{
		properties: *properties,
	}
	if l.unpackLen, err = readUint64LE(f); err != nil {
		return nil, err
	}
	if l.dict, err = newDecoderDict(bufferLen, historyLen); err != nil {
		return nil, err
	}
	l.posBitMask = (uint32(1) << uint(l.properties.PB)) - 1
	if l.rd, err = newRangeDecoder(f); err != nil {
		return nil, err
	}
	initProbSlice(l.isMatch[:])
	initProbSlice(l.isRep[:])
	initProbSlice(l.isRepG0[:])
	initProbSlice(l.isRepG1[:])
	initProbSlice(l.isRepG2[:])
	initProbSlice(l.isRepG0Long[:])
	l.litDecoder = newLiteralCodec(l.properties.LC, l.properties.LP)
	l.lengthDecoder = newLengthCodec()
	l.repLengthDecoder = newLengthCodec()
	l.distDecoder = newDistCodec()
	return l, nil
}

// Properties returns a set of properties.
func (l *Reader) Properties() Properties {
	return l.properties
}

// getUint64LE converts the uint64 value stored as little endian to an uint64
// value.
func getUint64LE(b []byte) uint64 {
	x := uint64(b[7]) << 56
	x |= uint64(b[6]) << 48
	x |= uint64(b[5]) << 40
	x |= uint64(b[4]) << 32
	x |= uint64(b[3]) << 24
	x |= uint64(b[2]) << 16
	x |= uint64(b[1]) << 8
	x |= uint64(b[0])
	return x
}

// readUint64LE reads a uint64 little-endian integer from reader.
func readUint64LE(r io.Reader) (x uint64, err error) {
	b := make([]byte, 8)
	if _, err = io.ReadFull(r, b); err != nil {
		return 0, err
	}
	x = getUint64LE(b)
	return x, nil
}

// initProbSlice initializes a slice of probabilities.
func initProbSlice(p []prob) {
	for i := range p {
		p[i] = probInit
	}
}

// Reads reads data from the decoder stream.
//
// The method might block and is not reentrant.
//
// The end of the LZMA stream is indicated by EOF. There might be other errors
// returned. The decoder will not be able to recover from an error returned.
func (l *Reader) Read(p []byte) (n int, err error) {
	for {
		var k int
		k, err = l.dict.Read(p[n:])
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
		if err = l.fill(); err != nil {
			return n, err
		}
	}
}

// errUnexpectedEOS indicates that the function decoded an unexpected end of
// stream marker
var errUnexpectedEOS = newError("unexpected end of stream marker")

// fill puts at lest the requested number of bytes into the decoder dictionary.
func (l *Reader) fill() error {
	if l.dict.eof {
		return nil
	}
	for l.dict.readable() < l.dict.b {
		op, err := l.decodeOp()
		if err != nil {
			switch {
			case err == eofDecoded:
				if l.unpackLen != noUnpackLen &&
					l.decodedLen != l.unpackLen {
					return errUnexpectedEOS
				}
				l.dict.eof = true
				return nil
			case err == io.EOF:
				return newError(
					"unexpected end of compressed stream")
			default:
				return err
			}
		}

		n := l.decodedLen + uint64(op.Len())
		if n < l.decodedLen {
			return newError(
				"negative op length or overflow of decodedLen")
		}
		if n > l.unpackLen {
			l.dict.eof = true
			return newError("decoded stream too long")
		}
		l.decodedLen = n

		if err = op.applyDecoderDict(l.dict); err != nil {
			return err
		}
		if n == l.unpackLen {
			l.dict.eof = true
			if !l.rd.possiblyAtEnd() {
				if _, err = l.decodeOp(); err != eofDecoded {
					return newError(
						"wrong length in header")
				}
			}
			return nil
		}
	}
	return nil
}

// updateStateLiteral updates the state for a literal.
func (l *Reader) updateStateLiteral() {
	switch {
	case l.state < 4:
		l.state = 0
		return
	case l.state < 10:
		l.state -= 3
		return
	}
	l.state -= 6
}

// updateStateMatch updates the state for a match.
func (l *Reader) updateStateMatch() {
	if l.state < 7 {
		l.state = 7
	} else {
		l.state = 10
	}
}

// updateStateRep updates the state for a repetition.
func (l *Reader) updateStateRep() {
	if l.state < 7 {
		l.state = 8
	} else {
		l.state = 11
	}
}

// updateStateShortRep updates the state for a short repetition.
func (l *Reader) updateStateShortRep() {
	if l.state < 7 {
		l.state = 9
	} else {
		l.state = 11
	}
}

var litCounter int

// decodeLiteral decodes a literal.
func (l *Reader) decodeLiteral() (op operation, err error) {
	prevByte := l.dict.getByte(1)
	lp, lc := uint(l.properties.LP), uint(l.properties.LC)
	litState := ((uint32(l.dict.total) & ((1 << lp) - 1)) << lc) |
		(uint32(prevByte) >> (8 - lc))

	litCounter++
	xlog.Printf(Debug, "L %3d %2d 0x%02x %3d\n", litCounter, litState,
		prevByte, l.dict.total)

	match := l.dict.getByte(int(l.rep[0]) + 1)
	s, err := l.litDecoder.Decode(l.rd, l.state, match, litState)
	if err != nil {
		return nil, err
	}
	return lit{s}, nil
}

// errWrongTermination indicates that a termination symbol has been received,
// but the range decoder could still produces more data
var errWrongTermination = newError(
	"end of stream marker at wrong place")

// eofDecoded indicates an EOF of the decoded file
var eofDecoded = newError("EOF of decoded stream")

var opCounter int

// decodeOp decodes an operation. The function returns eofDecoded if there is
// an explicit termination marker.
func (l *Reader) decodeOp() (op operation, err error) {
	posState := uint32(l.dict.total) & l.posBitMask

	opCounter++
	xlog.Printf(Debug, "S %3d %2d %2d\n", opCounter, l.state, posState)

	state2 := (l.state << maxPosBits) | posState

	b, err := l.isMatch[state2].Decode(l.rd)
	if err != nil {
		return nil, err
	}
	if b == 0 {
		// literal
		op, err := l.decodeLiteral()
		if err != nil {
			return nil, err
		}
		l.updateStateLiteral()
		return op, nil
	}
	b, err = l.isRep[l.state].Decode(l.rd)
	if err != nil {
		return nil, err
	}
	if b == 0 {
		// simple match
		l.rep[3], l.rep[2], l.rep[1] = l.rep[2], l.rep[1], l.rep[0]
		l.updateStateMatch()
		// The length decoder returns the length offset.
		n, err := l.lengthDecoder.Decode(l.rd, posState)
		if err != nil {
			return nil, err
		}
		// The dist decoder returns the distance offset. The actual
		// distance is 1 higher.
		l.rep[0], err = l.distDecoder.Decode(n, l.rd)
		if err != nil {
			return nil, err
		}
		if l.rep[0] == 0xffffffff {
			if !l.rd.possiblyAtEnd() {
				return nil, errWrongTermination
			}
			return nil, eofDecoded
		}
		op = rep{length: int(n) + minLength,
			distance: int(l.rep[0]) + minDistance}
		return op, nil
	}
	b, err = l.isRepG0[l.state].Decode(l.rd)
	if err != nil {
		return nil, err
	}
	dist := l.rep[0]
	if b == 0 {
		// rep match 0
		b, err = l.isRepG0Long[state2].Decode(l.rd)
		if err != nil {
			return nil, err
		}
		if b == 0 {
			l.updateStateShortRep()
			op = rep{length: 1,
				distance: int(l.rep[0]) + minDistance}
			return op, nil
		}
	} else {
		b, err = l.isRepG1[l.state].Decode(l.rd)
		if err != nil {
			return nil, err
		}
		if b == 0 {
			dist = l.rep[1]
		} else {
			b, err = l.isRepG2[l.state].Decode(l.rd)
			if err != nil {
				return nil, err
			}
			if b == 0 {
				dist = l.rep[2]
			} else {
				dist = l.rep[3]
				l.rep[3] = l.rep[2]
			}
			l.rep[2] = l.rep[1]
		}
		l.rep[1] = l.rep[0]
		l.rep[0] = dist
	}
	n, err := l.repLengthDecoder.Decode(l.rd, posState)
	if err != nil {
		return nil, err
	}
	l.updateStateRep()
	op = rep{length: int(n) + minLength, distance: int(dist) + minDistance}
	return op, nil
}
