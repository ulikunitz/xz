package lzma

import (
	"bufio"
	"io"
)

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
func (c *opCodec) init(p *Properties, dict dictionary) error {
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

// opReader provides an operation reader from an encoded source.
type opReader struct {
	opCodec
	rd *rangeDecoder
}

// newOpReader creates a new instance of an opReader.
func newOpReader(r io.Reader, p *Properties, dict dictionary) (or *opReader, err error) {
	or = new(opReader)
	if or.rd, err = newRangeDecoder(bufio.NewReader(r)); err != nil {
		return nil, err
	}
	if err = or.opCodec.init(p, dict); err != nil {
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

// Computes the states of the operation codec.
func (c *opCodec) states() (state, state2, posState uint32) {
	state = c.state
	posState = uint32(c.dict.Offset()) & c.posBitMask
	state2 = (c.state << maxPosBits) | posState
	return
}

func (c *opCodec) litState() uint32 {
	prevByte := c.dict.Byte(1)
	lp, lc := uint(c.properties.LP), uint(c.properties.LC)
	litState := ((uint32(c.dict.Offset())) & ((1 << lp) - 1) << lc) |
		(uint32(prevByte) >> (8 - lc))
	return litState
}

// decodeLiteral decodes a literal.
func (or *opReader) decodeLiteral() (op operation, err error) {
	litState := or.litState()

	match := or.dict.Byte(int(or.rep[0]) + 1)
	s, err := or.litCodec.Decode(or.rd, or.state, match, litState)
	if err != nil {
		return nil, err
	}
	return lit{s}, nil
}

// eos indicates an explicit end of stream
var eos = newError("end of decoded stream")

// ReadOp decodes the next operation from the compressed stream. It returns the
// operation. If an exlicit end of stream marker is identified the eos error is
// returned.
func (or *opReader) ReadOp() (op operation, err error) {
	state, state2, posState := or.states()

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
	b, err = or.isRep[state].Decode(or.rd)
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
		if or.rep[0] == eosDist {
			if !or.rd.possiblyAtEnd() {
				return nil, errWrongTermination
			}
			return nil, eos
		}
		op = rep{length: int(n) + minLength,
			distance: int64(or.rep[0]) + minDistance}
		return op, nil
	}
	b, err = or.isRepG0[state].Decode(or.rd)
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
				distance: int64(dist) + minDistance}
			return op, nil
		}
	} else {
		b, err = or.isRepG1[state].Decode(or.rd)
		if err != nil {
			return nil, err
		}
		if b == 0 {
			dist = or.rep[1]
		} else {
			b, err = or.isRepG2[state].Decode(or.rd)
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
	op = rep{length: int(n) + minLength,
		distance: int64(dist) + minDistance}
	return op, nil
}

// opWriter supports the writing of operations.
type opWriter struct {
	opCodec
	re *rangeEncoder
}

// newOpWriter creates a new operation writer.
func newOpWriter(w io.Writer, p *Properties, dict dictionary) (ow *opWriter, err error) {
	ow = new(opWriter)
	if ow.re = newRangeEncoder(bufio.NewWriter(w)); err != nil {
		return nil, err
	}
	if err = ow.opCodec.init(p, dict); err != nil {
		return nil, err
	}
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
	return ow.writeRep(rep{distance: maxDistance, length: minLength})
}

// writeRep writes a repetition operation into the operation stream
func (ow *opWriter) writeRep(r rep) error {
	var err error
	if !(minDistance <= r.distance && r.distance <= maxDistance) {
		return newError("distance out of range")
	}
	dist := uint32(r.distance - minDistance)
	if !(minLength <= r.length && r.length <= maxLength) &&
		!(dist == ow.rep[0] && r.length == 1) {
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
	n := uint32(r.length - minLength)
	if g > 4 {
		// simple match
		if err = ow.isRep[state].Encode(ow.re, 0); err != nil {
			return err
		}
		ow.rep[3], ow.rep[2], ow.rep[1], ow.rep[0] = ow.rep[2],
			ow.rep[1], ow.rep[0], dist
		ow.updateStateMatch()
		if err = ow.lenCodec.Encode(ow.re, n, posState); err != nil {
			return err
		}
		return ow.distCodec.Encode(ow.re, dist, n)
	}
	if err = ow.isRep[state].Encode(ow.re, 1); err != nil {
		return err
	}
	if g == 0 {
		if err = ow.isRepG0[state].Encode(ow.re, 0); err != nil {
			return err
		}
		if r.length == 1 {
			ow.updateStateShortRep()
			return ow.isRepG0Long[state2].Encode(ow.re, 0)
		}
	} else {
		if err = ow.isRepG0[state].Encode(ow.re, 1); err != nil {
			return err
		}
		if g == 1 {
			err = ow.isRepG1[state].Encode(ow.re, 0)
			if err != nil {
				return err
			}
		} else {
			err = ow.isRepG1[state].Encode(ow.re, 1)
			if err != nil {
				return err
			}
			if g == 2 {
				err = ow.isRepG2[state].Encode(ow.re, 0)
				if err != nil {
					return err
				}
			} else {
				err = ow.isRepG2[state].Encode(ow.re, 1)
				if err != nil {
					return err
				}
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
	case rep:
		return ow.writeRep(x)
	case lit:
		return ow.writeLiteral(x)
	}
	panic("unknown operation type")
}

// Close stops the operation writer. The range encoder is flushed out. The
// underlying writer is not closed.
func (ow *opWriter) Close() error {
	return ow.re.Flush()
}
