package lzma

import (
	"bufio"
	"io"
)

// states defines the overall state count
const states = 12

// dictHelper is an interface that provides the required interface to encode or
// decode operations successfully.
type dictHelper interface {
	GetByte(distance int) byte
	Total() int64
}

// opCodec provides all information to be able to encode or decode operations.
type opCodec struct {
	properties  Properties
	dict        dictHelper
	state       uint32
	posBitMask  uint32
	isMatch     [states << maxPosBits]prob
	isRep       [states]prob
	isRepG0     [states]prob
	isRepG1     [states]prob
	isRepG2     [states]prob
	isRepG0Long [states << maxPosBits]prob
	rep         [4]uint32
	litCodec    *literalCodec
	lenCodec    *lengthCodec
	repLenCodec *lengthCodec
	distCodec   *distCodec
}

// initProbSlice initializes a slice of probabilities.
func initProbSlice(p []prob) {
	for i := range p {
		p[i] = probInit
	}
}

// initOpCodec initializes an opCodec structure.
func initOpCodec(c *opCodec, p *Properties, dict dictHelper) error {
	var err error
	if err = verifyProperties(p); err != nil {
		return err
	}
	c.properties = *p
	c.dict = dict
	c.posBitMask = (uint32(1) << uint(c.properties.PB)) - 1
	initProbSlice(c.isMatch[:])
	initProbSlice(c.isRep[:])
	initProbSlice(c.isRepG0[:])
	initProbSlice(c.isRepG1[:])
	initProbSlice(c.isRepG2[:])
	initProbSlice(c.isRepG0Long[:])
	c.litCodec = newLiteralCodec(c.properties.LC, c.properties.LP)
	c.lenCodec = newLengthCodec()
	c.repLenCodec = newLengthCodec()
	c.distCodec = newDistCodec()
	return nil
}

// Properties returns the properites stored in the opCodec structure.
func (c *opCodec) Properties() Properties {
	return c.properties
}

// OpReader provides an operation reader from an encoded source.
type opReader struct {
	opCodec
	rd *rangeDecoder
}

// newOpReader creates a new instance of an opReader.
func newOpReader(r io.Reader, p *Properties, dict dictHelper) (or *opReader, err error) {
	or = new(opReader)
	if or.rd, err = newRangeDecoder(bufio.NewReader(r)); err != nil {
		return nil, err
	}
	if err = initOpCodec(&or.opCodec, p, dict); err != nil {
		return nil, err
	}
	return or, nil
}

// updateStateLiteral updates the state for a literal.
func (c *opCodec) updateStateLiteral() {
	switch {
	case c.state < 4:
		c.state = 0
		return
	case c.state < 10:
		c.state -= 3
		return
	}
	c.state -= 6
}

// updateStateMatch updates the state for a match.
func (c *opCodec) updateStateMatch() {
	if c.state < 7 {
		c.state = 7
	} else {
		c.state = 10
	}
}

// updateStateRep updates the state for a repetition.
func (c *opCodec) updateStateRep() {
	if c.state < 7 {
		c.state = 8
	} else {
		c.state = 11
	}
}

// updateStateShortRep updates the state for a short repetition.
func (c *opCodec) updateStateShortRep() {
	if c.state < 7 {
		c.state = 9
	} else {
		c.state = 11
	}
}

// decodeLiteral decodes a literal.
func (or *opReader) decodeLiteral() (op operation, err error) {
	prevByte := or.dict.GetByte(1)
	lp, lc := uint(or.properties.LP), uint(or.properties.LC)
	litState := ((uint32(or.dict.Total()) & ((1 << lp) - 1)) << lc) |
		(uint32(prevByte) >> (8 - lc))

	match := or.dict.GetByte(int(or.rep[0]) + 1)
	s, err := or.litCodec.Decode(or.rd, or.state, match, litState)
	if err != nil {
		return nil, err
	}
	return lit{s}, nil
}

// eofDecoded indicates an EOF of the decoded file
var eofDecoded = newError("EOF of decoded stream")

func (or *opReader) ReadOp() (op operation, err error) {
	posState := uint32(or.dict.Total()) & or.posBitMask

	state2 := (or.state << maxPosBits) | posState

	b, err := or.isMatch[state2].Decode(or.rd)
	if err != nil {
		return nil, err
	}
	if b == 0 {
		// literal
		op, err := or.decodeLiteral()
		if err != nil {
			return nil, err
		}
		or.updateStateLiteral()
		return op, nil
	}
	b, err = or.isRep[or.state].Decode(or.rd)
	if err != nil {
		return nil, err
	}
	if b == 0 {
		// simple match
		or.rep[3], or.rep[2], or.rep[1] = or.rep[2], or.rep[1], or.rep[0]
		or.updateStateMatch()
		// The length decoder returns the length offset.
		n, err := or.lenCodec.Decode(or.rd, posState)
		if err != nil {
			return nil, err
		}
		// The dist decoder returns the distance offset. The actual
		// distance is 1 higher.
		or.rep[0], err = or.distCodec.Decode(or.rd, n)
		if err != nil {
			return nil, err
		}
		if or.rep[0] == eos {
			if !or.rd.possiblyAtEnd() {
				return nil, errWrongTermination
			}
			return nil, eofDecoded
		}
		op = rep{length: int(n) + minLength,
			distance: int(or.rep[0]) + minDistance}
		return op, nil
	}
	b, err = or.isRepG0[or.state].Decode(or.rd)
	if err != nil {
		return nil, err
	}
	dist := or.rep[0]
	if b == 0 {
		// rep match 0
		b, err = or.isRepG0Long[state2].Decode(or.rd)
		if err != nil {
			return nil, err
		}
		if b == 0 {
			or.updateStateShortRep()
			op = rep{length: 1,
				distance: int(dist) + minDistance}
			return op, nil
		}
	} else {
		b, err = or.isRepG1[or.state].Decode(or.rd)
		if err != nil {
			return nil, err
		}
		if b == 0 {
			dist = or.rep[1]
		} else {
			b, err = or.isRepG2[or.state].Decode(or.rd)
			if err != nil {
				return nil, err
			}
			if b == 0 {
				dist = or.rep[2]
			} else {
				dist = or.rep[3]
				or.rep[3] = or.rep[2]
			}
			or.rep[2] = or.rep[1]
		}
		or.rep[1] = or.rep[0]
		or.rep[0] = dist
	}
	n, err := or.repLenCodec.Decode(or.rd, posState)
	if err != nil {
		return nil, err
	}
	or.updateStateRep()
	op = rep{length: int(n) + minLength, distance: int(dist) + minDistance}
	return op, nil
}
