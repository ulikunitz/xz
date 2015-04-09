package lzbase

// states defines the overall state count
const states = 12

// Value of the end of stream (EOS) marker.
const eosDist = 1<<32 - 1

// Dictionary abstracts the concrete dictionary away
type Dictionary interface {
	Byte(dist int64) byte
	Offset() int64
}

// OpCodec provides all information to be able to encode or decode operations.
type OpCodec struct {
	Properties  Properties
	dict        Dictionary
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

// init initializes an OpCodec structure.
func (c *OpCodec) init(p Properties, dict Dictionary) {
	c.Properties = p
	c.dict = dict
	c.posBitMask = (uint32(1) << uint(c.Properties.PB())) - 1
	initProbSlice(c.isMatch[:])
	initProbSlice(c.isRep[:])
	initProbSlice(c.isRepG0[:])
	initProbSlice(c.isRepG1[:])
	initProbSlice(c.isRepG2[:])
	initProbSlice(c.isRepG0Long[:])
	c.litCodec = newLiteralCodec(c.Properties.LC(), c.Properties.LP())
	c.lenCodec = newLengthCodec()
	c.repLenCodec = newLengthCodec()
	c.distCodec = newDistCodec()
}

// NewOpCodec creates a new OpCodec.
func NewOpCodec(p Properties, dict Dictionary) *OpCodec {
	oc := new(OpCodec)
	oc.init(p, dict)
	return oc
}

// updateStateLiteral updates the state for a literal.
func (c *OpCodec) updateStateLiteral() {
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
func (c *OpCodec) updateStateMatch() {
	if c.state < 7 {
		c.state = 7
	} else {
		c.state = 10
	}
}

// updateStateRep updates the state for a repetition.
func (c *OpCodec) updateStateRep() {
	if c.state < 7 {
		c.state = 8
	} else {
		c.state = 11
	}
}

// updateStateShortRep updates the state for a short repetition.
func (c *OpCodec) updateStateShortRep() {
	if c.state < 7 {
		c.state = 9
	} else {
		c.state = 11
	}
}

// Computes the states of the operation codec.
func (c *OpCodec) states() (state, state2, posState uint32) {
	state = c.state
	posState = uint32(c.dict.Offset()) & c.posBitMask
	state2 = (c.state << maxPosBits) | posState
	return
}

func (c *OpCodec) litState() uint32 {
	prevByte := c.dict.Byte(1)
	lp, lc := uint(c.Properties.LP()), uint(c.Properties.LC())
	litState := ((uint32(c.dict.Offset())) & ((1 << lp) - 1) << lc) |
		(uint32(prevByte) >> (8 - lc))
	return litState
}
