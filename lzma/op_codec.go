package lzma

// states defines the overall state count
const states = 12

type dictHelper interface {
	GetByte(distance int) byte
	Total() int64
}

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

func newOpCodec(p *Properties, dict dictHelper) (c *opCodec, err error) {
	if err = verifyProperties(p); err != nil {
		return nil, err
	}
	c = &opCodec{properties: *p, dict: dict}
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
	return c, nil
}

func (c *opCodec) Properties() Properties {
	return c.properties
}

func (c *opCodec) Encode(e *rangeEncoder, op operation) error {
	panic("TODO")
}

// Close closes the output stream. It writes the end-of-stream marker if
// required and flushes the range encoder.
func (c *opCodec) Close(e *rangeEncoder, eos bool) error {
	panic("TODO")
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
func (c *opCodec) decodeLiteral(d *rangeDecoder) (op operation, err error) {
	prevByte := c.dict.GetByte(1)
	lp, lc := uint(c.properties.LP), uint(c.properties.LC)
	litState := ((uint32(c.dict.Total()) & ((1 << lp) - 1)) << lc) |
		(uint32(prevByte) >> (8 - lc))

	match := c.dict.GetByte(int(c.rep[0]) + 1)
	s, err := c.litCodec.Decode(d, c.state, match, litState)
	if err != nil {
		return nil, err
	}
	return lit{s}, nil
}

// eofDecoded indicates an EOF of the decoded file
var eofDecoded = newError("EOF of decoded stream")

func (c *opCodec) Decode(d *rangeDecoder) (op operation, err error) {
	posState := uint32(c.dict.Total()) & c.posBitMask

	state2 := (c.state << maxPosBits) | posState

	b, err := c.isMatch[state2].Decode(d)
	if err != nil {
		return nil, err
	}
	if b == 0 {
		// literal
		op, err := c.decodeLiteral(d)
		if err != nil {
			return nil, err
		}
		c.updateStateLiteral()
		return op, nil
	}
	b, err = c.isRep[c.state].Decode(d)
	if err != nil {
		return nil, err
	}
	if b == 0 {
		// simple match
		c.rep[3], c.rep[2], c.rep[1] = c.rep[2], c.rep[1], c.rep[0]
		c.updateStateMatch()
		// The length decoder returns the length offset.
		n, err := c.lenCodec.Decode(d, posState)
		if err != nil {
			return nil, err
		}
		// The dist decoder returns the distance offset. The actual
		// distance is 1 higher.
		c.rep[0], err = c.distCodec.Decode(n, d)
		if err != nil {
			return nil, err
		}
		if c.rep[0] == eos {
			if !d.possiblyAtEnd() {
				return nil, errWrongTermination
			}
			return nil, eofDecoded
		}
		op = rep{length: int(n) + minLength,
			distance: int(c.rep[0]) + minDistance}
		return op, nil
	}
	b, err = c.isRepG0[c.state].Decode(d)
	if err != nil {
		return nil, err
	}
	dist := c.rep[0]
	if b == 0 {
		// rep match 0
		b, err = c.isRepG0Long[state2].Decode(d)
		if err != nil {
			return nil, err
		}
		if b == 0 {
			c.updateStateShortRep()
			op = rep{length: 1,
				distance: int(dist) + minDistance}
			return op, nil
		}
	} else {
		b, err = c.isRepG1[c.state].Decode(d)
		if err != nil {
			return nil, err
		}
		if b == 0 {
			dist = c.rep[1]
		} else {
			b, err = c.isRepG2[c.state].Decode(d)
			if err != nil {
				return nil, err
			}
			if b == 0 {
				dist = c.rep[2]
			} else {
				dist = c.rep[3]
				c.rep[3] = c.rep[2]
			}
			c.rep[2] = c.rep[1]
		}
		c.rep[1] = c.rep[0]
		c.rep[0] = dist
	}
	n, err := c.repLenCodec.Decode(d, posState)
	if err != nil {
		return nil, err
	}
	c.updateStateRep()
	op = rep{length: int(n) + minLength, distance: int(dist) + minDistance}
	return op, nil
}
