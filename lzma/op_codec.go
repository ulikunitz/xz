package lzma

import "io"

// states defines the overall state count
const states = 12

// Value of the end of stream (EOS) marker.
const eosDist = 1<<32 - 1

// dictionary abstracts the concrete dictionary away
type dictionary interface {
	Byte(dist int) byte
	Offset() int64
}

// opCodec provides all information to be able to encode or decode operations.
type opCodec struct {
	properties  Properties
	dict        dictionary
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

// init initializes an opCodec structure.
func (c *opCodec) init(p Properties, dict dictionary) {
	c.properties = p
	c.dict = dict
	c.posBitMask = (uint32(1) << uint(c.properties.PB())) - 1
	initProbSlice(c.isMatch[:])
	initProbSlice(c.isRep[:])
	initProbSlice(c.isRepG0[:])
	initProbSlice(c.isRepG1[:])
	initProbSlice(c.isRepG2[:])
	initProbSlice(c.isRepG0Long[:])
	c.litCodec = newLiteralCodec(c.properties.LC(), c.properties.LP())
	c.lenCodec = newLengthCodec()
	c.repLenCodec = newLengthCodec()
	c.distCodec = newDistCodec()
}

// opReader provides an operation reader from an encoded source.
type opReader struct {
	opCodec
	rd *rangeDecoder
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

// Computes the states of the operation codec.
func (c *opCodec) states() (state, state2, posState uint32) {
	state = c.state
	posState = uint32(c.dict.Offset()) & c.posBitMask
	state2 = (c.state << maxPosBits) | posState
	return
}

func (c *opCodec) litState() uint32 {
	prevByte := c.dict.Byte(1)
	lp, lc := uint(c.properties.LP()), uint(c.properties.LC())
	litState := ((uint32(c.dict.Offset())) & ((1 << lp) - 1) << lc) |
		(uint32(prevByte) >> (8 - lc))
	return litState
}

// opWriter supports the writing of operations.
type opWriter struct {
	opCodec
	re *rangeEncoder
}

// newOpWriter creates a new operation writer.
func newOpWriter(w io.Writer, p *Parameters, dict dictionary) (ow *opWriter, err error) {
	ow = new(opWriter)
	if ow.re = newRangeEncoder(w); err != nil {
		return nil, err
	}
	ow.opCodec.init(p.Properties(), dict)
	return ow, nil
}

// writeLiteral writes a literal into the operation stream
func (ow *opWriter) writeLiteral(l lit) error {
	var err error
	state, state2, _ := ow.states()
	if err = ow.isMatch[state2].Encode(ow.re, 0); err != nil {
		return err
	}
	litState := ow.litState()
	match := ow.dict.Byte(int(ow.rep[0]) + 1)
	err = ow.litCodec.Encode(ow.re, l.b, state, match, litState)
	if err != nil {
		return err
	}
	ow.updateStateLiteral()
	return nil
}

// writeEOS writes the explicit EOS marker
func (ow *opWriter) writeEOS() error {
	return ow.writeMatch(match{distance: maxDistance, length: minLength})
}

func iverson(ok bool) uint32 {
	if ok {
		return 1
	}
	return 0
}

// writeRep writes a repetition operation into the operation stream
func (ow *opWriter) writeMatch(m match) error {
	var err error
	if !(minDistance <= m.distance && m.distance <= maxDistance) {
		return newError("distance out of range")
	}
	dist := uint32(m.distance - minDistance)
	if !(minLength <= m.length && m.length <= maxLength) &&
		!(dist == ow.rep[0] && m.length == 1) {
		return newError("length out of range")
	}
	state, state2, posState := ow.states()
	if err = ow.isMatch[state2].Encode(ow.re, 1); err != nil {
		return err
	}
	var g int
	for g = 0; g < 4; g++ {
		if ow.rep[g] == dist {
			break
		}
	}
	b := iverson(g < 4)
	if err = ow.isRep[state].Encode(ow.re, b); err != nil {
		return err
	}
	n := uint32(m.length - minLength)
	if b == 0 {
		// simple match
		ow.rep[3], ow.rep[2], ow.rep[1], ow.rep[0] = ow.rep[2],
			ow.rep[1], ow.rep[0], dist
		ow.updateStateMatch()
		if err = ow.lenCodec.Encode(ow.re, n, posState); err != nil {
			return err
		}
		return ow.distCodec.Encode(ow.re, dist, n)
	}
	b = iverson(g != 0)
	if err = ow.isRepG0[state].Encode(ow.re, b); err != nil {
		return err
	}
	if b == 0 {
		// g == 0
		b = iverson(m.length != 1)
		if err = ow.isRepG0Long[state2].Encode(ow.re, b); err != nil {
			return err
		}
		if b == 0 {
			ow.updateStateShortRep()
			return nil
		}
	} else {
		// g in {1,2,3}
		b = iverson(g != 1)
		if err = ow.isRepG1[state].Encode(ow.re, b); err != nil {
			return err
		}
		if b == 1 {
			// g in {2,3}
			b = iverson(g != 2)
			err = ow.isRepG2[state].Encode(ow.re, b)
			if err != nil {
				return err
			}
			if b == 1 {
				ow.rep[3] = ow.rep[2]
			}
			ow.rep[2] = ow.rep[1]
		}
		ow.rep[1] = ow.rep[0]
		ow.rep[0] = dist
	}
	ow.updateStateRep()
	return ow.repLenCodec.Encode(ow.re, n, posState)
}

// WriteOp writes an operation value into the stream.
func (ow *opWriter) WriteOp(op operation) error {
	switch x := op.(type) {
	case match:
		return ow.writeMatch(x)
	case lit:
		return ow.writeLiteral(x)
	}
	panic("unknown operation type")
}

// Close stops the operation writer. The range encoder is flushed out. The
// underlying writer is not closed.
func (ow *opWriter) Close() error {
	return ow.re.Close()
}
