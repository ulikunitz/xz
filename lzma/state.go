package lzma

import (
	"errors"
	"io"
	"math/bits"
)

// number of supported states
const states = 12

// maxPosBits defines the number of bits of the position value that are used to
// to compute the posState value. The value is used to select the tree codec
// for length encoding and decoding.
const maxPosBits = 4

type state1Probs struct {
	isRep   prob
	isRepG0 prob
	isRepG1 prob
	isRepG2 prob
}

func initS1Probs(p []state1Probs) {
	for i := range p {
		p[i] = state1Probs{probInit, probInit, probInit, probInit}
	}
}

type state2Probs struct {
	isMatch     prob
	isRepG0Long prob
}

func initS2Probs(p []state2Probs) {
	for i := range p {
		p[i] = state2Probs{probInit, probInit}
	}
}

type state struct {
	s1          [states]state1Probs
	s2          [states << maxPosBits]state2Probs
	litCodec    literalCodec
	lenCodec    lengthCodec
	repLenCodec lengthCodec
	distCodec   distCodec
	Properties
	rep        [4]uint32
	state      uint32
	posBitMask uint32
}

func (s *state) init(p Properties) {
	*s = state{Properties: p}
	s.reset()
}

func (s *state) reset() {
	p := s.Properties
	*s = state{
		Properties: p,
		posBitMask: (1 << p.PB) - 1,
	}
	initS1Probs(s.s1[:])
	initS2Probs(s.s2[:])
	s.litCodec.init(p.LC, p.LP)
	s.lenCodec.init()
	s.repLenCodec.init()
	s.distCodec.init()
}

/*
func (s *state) deepCopy(src *state) {
	if s == src {
		return
	}
	*s = *src
	s.litCodec.deepCopy(&src.litCodec)
	s.lenCodec.deepCopy(&src.lenCodec)
	s.repLenCodec.deepCopy(&src.repLenCodec)
	s.distCodec.deepCopy(&src.distCodec)
}
*/

// updateStateLiteral updates the state for a literal.
func (s *state) updateStateLiteral() {
	switch {
	case s.state < 4:
		s.state = 0
		return
	case s.state < 10:
		s.state -= 3
		return
	}
	s.state -= 6
}

// updateStateMatch updates the state for a match.
func (s *state) updateStateMatch() {
	if s.state < 7 {
		s.state = 7
	} else {
		s.state = 10
	}
}

// updateStateRep updates the state for a repetition.
func (s *state) updateStateRep() {
	if s.state < 7 {
		s.state = 8
	} else {
		s.state = 11
	}
}

// updateStateShortRep updates the state for a short repetition.
func (s *state) updateStateShortRep() {
	if s.state < 7 {
		s.state = 9
	} else {
		s.state = 11
	}
}

// states computes the states of the operation codec.
func (s *state) states(dictHead int64) (state1, state2, posState uint32) {
	state1 = s.state
	posState = uint32(dictHead) & s.posBitMask
	state2 = (s.state << maxPosBits) | posState
	return
}

// litState computes the literal state.
func (s *state) litState(prev byte, dictHead int64) uint32 {
	litState := ((uint32(dictHead) & ((1 << s.LP) - 1)) << s.LC) |
		(uint32(prev) >> (8 - s.LC))
	return litState
}

// moveBits defines the number of bits used for the updates of probability
// values.
const moveBits = 5

// probBits defines the number of bits of a probability value.
const probBits = 11

// probInit defines 0.5 as initial value for prob values.
const probInit prob = 1 << (probBits - 1)

// Type prob represents probabilities. The type can also be used to encode and
// decode single bits.
type prob uint16

// Dec decreases the probability. The decrease is proportional to the
// probability value.
func (p *prob) dec() {
	*p -= *p >> moveBits
}

// Inc increases the probability. The Increase is proportional to the
// difference of 1 and the probability value.
func (p *prob) inc() {
	*p += ((1 << probBits) - *p) >> moveBits
}

// Computes the new bound for a given range using the probability value.
func (p prob) bound(r uint32) uint32 {
	return (r >> probBits) * uint32(p)
}

// Bits returns 1. One is the number of bits that can be encoded or decoded
// with a single prob value.
func (p prob) Bits() int {
	return 1
}

// minMatchLen and maxMatchLen give the minimum and maximum values for
// encoding and decoding length values. minMatchLen is also used as base
// for the encoded length values.
const (
	minMatchLen = 2
	maxMatchLen = minMatchLen + 16 + 256 - 1
)

// lengthCodec support the encoding of the length value.
type lengthCodec struct {
	choice [2]prob
	low    [1 << maxPosBits]treeCodec
	mid    [1 << maxPosBits]treeCodec
	high   treeCodec
}

/*
// deepCopy initializes the lc value as deep copy of the source value.
func (lc *lengthCodec) deepCopy(src *lengthCodec) {
	if lc == src {
		return
	}
	lc.choice = src.choice
	for i := range lc.low {
		lc.low[i].deepCopy(&src.low[i])
	}
	for i := range lc.mid {
		lc.mid[i].deepCopy(&src.mid[i])
	}
	lc.high.deepCopy(&src.high)
}
*/

// init initializes a new length codec.
func (lc *lengthCodec) init() {
	for i := range lc.choice {
		lc.choice[i] = probInit
	}
	for i := range lc.low {
		lc.low[i] = makeTreeCodec(3)
	}
	for i := range lc.mid {
		lc.mid[i] = makeTreeCodec(3)
	}
	lc.high = makeTreeCodec(8)
}

// Encode encodes the length offset. The length offset l can be compute by
// subtracting minMatchLen (2) from the actual length.
//
//   l = length - minMatchLen
//
func (lc *lengthCodec) Encode(e *rangeEncoder, l uint32, posState uint32,
) (err error) {
	if l > maxMatchLen-minMatchLen {
		return errors.New("lengthCodec.Encode: l out of range")
	}
	if l < 8 {
		if err = e.EncodeBit(0, &lc.choice[0]); err != nil {
			return
		}
		return lc.low[posState].Encode(e, l)
	}
	if err = e.EncodeBit(1, &lc.choice[0]); err != nil {
		return
	}
	if l < 16 {
		if err = e.EncodeBit(0, &lc.choice[1]); err != nil {
			return
		}
		return lc.mid[posState].Encode(e, l-8)
	}
	if err = e.EncodeBit(1, &lc.choice[1]); err != nil {
		return
	}
	if err = lc.high.Encode(e, l-16); err != nil {
		return
	}
	return nil
}

// Decode reads the length offset. Add minMatchLen to compute the actual length
// to the length offset l.
func (lc *lengthCodec) Decode(d *rangeDecoder, posState uint32,
) (l uint32, err error) {
	var b uint32
	b, err = d.decodeBit(&lc.choice[0])
	if err != nil {
		return
	}
	if b == 0 {
		l, err = lc.low[posState].Decode(d)
		return
	}
	b, err = d.decodeBit(&lc.choice[1])
	if err != nil {
		return
	}
	if b == 0 {
		l, err = lc.mid[posState].Decode(d)
		l += 8
		return
	}
	l, err = lc.high.Decode(d)
	l += 16
	return
}

// treeCodec encodes or decodes values with a fixed bit size. It is using a
// tree of probability value. The root of the tree is the most-significant bit.
type treeCodec struct {
	probTree
}

// makeTreeCodec makes a tree codec. The bits value must be inside the range
// [1,32].
func makeTreeCodec(bits int) treeCodec {
	return treeCodec{makeProbTree(bits)}
}

/*
// deepCopy initializes tc as a deep copy of the source.
func (tc *treeCodec) deepCopy(src *treeCodec) {
	tc.probTree.deepCopy(&src.probTree)
}
*/

// Encode uses the range encoder to encode a fixed-bit-size value.
func (tc *treeCodec) Encode(e *rangeEncoder, v uint32) (err error) {
	m := uint32(1)
	for i := int(tc.bits) - 1; i >= 0; i-- {
		b := (v >> uint(i)) & 1
		if err := e.EncodeBit(b, &tc.probs[m]); err != nil {
			return err
		}
		m = (m << 1) | b
	}
	return nil
}

// Decodes uses the range decoder to decode a fixed-bit-size value. Errors may
// be caused by the range decoder.
func (tc *treeCodec) Decode(d *rangeDecoder) (v uint32, err error) {
	m := uint32(1)
	for j := 0; j < int(tc.bits); j++ {
		b, err := d.decodeBit(&tc.probs[m])
		if err != nil {
			return 0, err
		}
		m = (m << 1) | b
	}
	return m - (1 << uint(tc.bits)), nil
}

// treeReverseCodec is another tree codec, where the least-significant bit is
// the start of the probability tree.
type treeReverseCodec struct {
	probTree
}

/*
// deepCopy initializes the treeReverseCodec as a deep copy of the
// source.
func (tc *treeReverseCodec) deepCopy(src *treeReverseCodec) {
	tc.probTree.deepCopy(&src.probTree)
}
*/

// makeTreeReverseCodec creates treeReverseCodec value. The bits argument must
// be in the range [1,32].
func makeTreeReverseCodec(bits int) treeReverseCodec {
	return treeReverseCodec{makeProbTree(bits)}
}

// Encode uses range encoder to encode a fixed-bit-size value. The range
// encoder may cause errors.
func (tc *treeReverseCodec) Encode(v uint32, e *rangeEncoder) (err error) {
	m := uint32(1)
	for i := uint(0); i < uint(tc.bits); i++ {
		b := (v >> i) & 1
		if err := e.EncodeBit(b, &tc.probs[m]); err != nil {
			return err
		}
		m = (m << 1) | b
	}
	return nil
}

// Decodes uses the range decoder to decode a fixed-bit-size value. Errors
// returned by the range decoder will be returned.
func (tc *treeReverseCodec) Decode(d *rangeDecoder) (v uint32, err error) {
	m := uint32(1)
	for j := uint(0); j < uint(tc.bits); j++ {
		b, err := d.decodeBit(&tc.probs[m])
		if err != nil {
			return 0, err
		}
		m = (m << 1) | b
		v |= b << j
	}
	return v, nil
}

// probTree stores enough probability values to be used by the treeEncode and
// treeDecode methods of the range coder types.
type probTree struct {
	probs []prob
	bits  byte
}

/*
// deepCopy initializes the probTree value as a deep copy of the source.
func (t *probTree) deepCopy(src *probTree) {
	if t == src {
		return
	}
	t.probs = make([]prob, len(src.probs))
	copy(t.probs, src.probs)
	t.bits = src.bits
}
*/

// makeProbTree initializes a probTree structure.
func makeProbTree(bits int) probTree {
	if !(1 <= bits && bits <= 32) {
		panic("bits outside of range [1,32]")
	}
	t := probTree{
		bits:  byte(bits),
		probs: make([]prob, 1<<uint(bits)),
	}
	for i := range t.probs {
		t.probs[i] = probInit
	}
	return t
}

// Bits provides the number of bits for the values to de- or encode.
func (t *probTree) Bits() int {
	return int(t.bits)
}

// rangeDecoder decodes single bits of the range encoding stream.
type rangeDecoder struct {
	br     io.ByteReader
	nrange uint32
	code   uint32
}

// init initializes the rangeDecoder. It reads five bytes from the stream and
// may return errors.
func (d *rangeDecoder) init(br io.ByteReader) error {
	*d = rangeDecoder{br: br, nrange: 0xffffffff}

	b, err := d.br.ReadByte()
	if err != nil {
		return err
	}
	if b != 0 {
		return errors.New("lzma: first byte of LZMA stream not zero")
	}
	for i := 0; i < 4; i++ {
		if err = d.updateCode(); err != nil {
			return err
		}
	}
	if d.code >= d.nrange {
		return errors.New("lzma: d.code >= d.nrange")
	}
	return nil
}

// possiblyAtEnd checks whether the decoder may be at the end of the stream.
func (d *rangeDecoder) possiblyAtEnd() bool {
	return d.code == 0
}

// directDecodeBit decodes a bit with probability 1/2. The return value b will
// contain the bit at the least-significant position. All other bits will be
// zero.
func (d *rangeDecoder) directDecodeBit() (b uint32, err error) {
	d.nrange >>= 1
	d.code -= d.nrange
	t := 0 - (d.code >> 31)
	d.code += d.nrange & t
	b = (t + 1) & 1

	// d.code will stay less then d.nrange

	// normalize
	// assume d.code < d.nrange
	const top = 1 << 24
	if d.nrange >= top {
		return b, nil
	}
	d.nrange <<= 8
	// d.code < d.nrange will be maintained
	return b, d.updateCode()
}

// decodeBit decodes a single bit. The bit will be returned at the
// least-significant position. All other bits will be zero. The probability
// value will be updated.
func (d *rangeDecoder) decodeBit(p *prob) (b uint32, err error) {
	bound := p.bound(d.nrange)
	if d.code < bound {
		d.nrange = bound
		p.inc()
		b = 0
	} else {
		d.code -= bound
		d.nrange -= bound
		p.dec()
		b = 1
	}
	// normalize
	// assume d.code < d.nrange
	const top = 1 << 24
	if d.nrange >= top {
		return b, nil
	}
	d.nrange <<= 8
	// d.code < d.nrange will be maintained
	return b, d.updateCode()
}

// updateCode reads a new byte into the code.
func (d *rangeDecoder) updateCode() error {
	b, err := d.br.ReadByte()
	if err != nil {
		return err
	}
	d.code = (d.code << 8) | uint32(b)
	return nil
}

// literalCodec supports the encoding of literal. It provides 768 probability
// values per literal state. The upper 512 probabilities are used with the
// context of a match bit.
type literalCodec struct {
	probs []prob
}

/*
// deepCopy initializes literal codec c as a deep copy of the source.
func (c *literalCodec) deepCopy(src *literalCodec) {
	if c == src {
		return
	}
	c.probs = make([]prob, len(src.probs))
	copy(c.probs, src.probs)
}
*/

// init initializes the literal codec.
func (c *literalCodec) init(lc, lp int) {
	switch {
	case !(minLC <= lc && lc <= maxLC):
		panic("lc out of range")
	case !(minLP <= lp && lp <= maxLP):
		panic("lp out of range")
	}
	c.probs = make([]prob, 0x300<<uint(lc+lp))
	for i := range c.probs {
		c.probs[i] = probInit
	}
}

// Encode encodes the byte s using a range encoder as well as the current LZMA
// encoder state, a match byte and the literal state.
func (c *literalCodec) Encode(e *rangeEncoder, s byte,
	state uint32, match byte, litState uint32,
) (err error) {
	k := litState * 0x300
	probs := c.probs[k : k+0x300]
	symbol := uint32(1)
	r := uint32(s)
	if state >= 7 {
		m := uint32(match)
		for {
			matchBit := (m >> 7) & 1
			m <<= 1
			bit := (r >> 7) & 1
			r <<= 1
			i := ((1 + matchBit) << 8) | symbol
			err = e.EncodeBit(bit, &probs[i])
			if err != nil {
				return
			}
			symbol = (symbol << 1) | bit
			if matchBit != bit {
				break
			}
			if symbol >= 0x100 {
				break
			}
		}
	}
	for symbol < 0x100 {
		bit := (r >> 7) & 1
		r <<= 1
		err = e.EncodeBit(bit, &probs[symbol])
		if err != nil {
			return
		}
		symbol = (symbol << 1) | bit
	}
	return nil
}

// Decode decodes a literal byte using the range decoder as well as the LZMA
// state, a match byte, and the literal state.
func (c *literalCodec) Decode(d *rangeDecoder,
	state uint32, match byte, litState uint32,
) (s byte, err error) {
	k := litState * 0x300
	probs := c.probs[k : k+0x300]
	symbol := uint32(1)
	if state >= 7 {
		m := uint32(match)
		for {
			matchBit := (m >> 7) & 1
			m <<= 1
			i := ((1 + matchBit) << 8) | symbol
			bit, err := d.decodeBit(&probs[i])
			if err != nil {
				return 0, err
			}
			symbol = (symbol << 1) | bit
			if matchBit != bit {
				break
			}
			if symbol >= 0x100 {
				break
			}
		}
	}
	for symbol < 0x100 {
		bit, err := d.decodeBit(&probs[symbol])
		if err != nil {
			return 0, err
		}
		symbol = (symbol << 1) | bit
	}
	s = byte(symbol - 0x100)
	return s, nil
}

// minLC and maxLC define the range for LC values.
const (
	minLC = 0
	maxLC = 8
)

// minLC and maxLC define the range for LP values.
const (
	minLP = 0
	maxLP = 4
)

// Constants used by the distance codec.
const (
	// minimum supported distance
	minDistance = 1
	// maximum supported distance, value is used for the eos marker.
	maxDistance = 1<<32 - 1
	// number of the supported len states
	lenStates = 4
	// start for the position models
	startPosModel = 4
	// first index with align bits support
	endPosModel = 14
	// bits for the position slots
	posSlotBits = 6
	// number of align bits
	alignBits = 4
)

// distCodec provides encoding and decoding of distance values.
type distCodec struct {
	posSlotCodecs [lenStates]treeCodec
	posModel      [endPosModel - startPosModel]treeReverseCodec
	alignCodec    treeReverseCodec
}

/*
// deepCopy initializes dc as deep copy of the source.
func (dc *distCodec) deepCopy(src *distCodec) {
	if dc == src {
		return
	}
	for i := range dc.posSlotCodecs {
		dc.posSlotCodecs[i].deepCopy(&src.posSlotCodecs[i])
	}
	for i := range dc.posModel {
		dc.posModel[i].deepCopy(&src.posModel[i])
	}
	dc.alignCodec.deepCopy(&src.alignCodec)
}
*/

// newDistCodec creates a new distance codec.
func (dc *distCodec) init() {
	for i := range dc.posSlotCodecs {
		dc.posSlotCodecs[i] = makeTreeCodec(posSlotBits)
	}
	for i := range dc.posModel {
		posSlot := startPosModel + i
		bits := (posSlot >> 1) - 1
		dc.posModel[i] = makeTreeReverseCodec(bits)
	}
	dc.alignCodec = makeTreeReverseCodec(alignBits)
}

// lenState converts the value l to a supported lenState value.
func lenState(l uint32) uint32 {
	if l >= lenStates {
		l = lenStates - 1
	}
	return l
}

// Encode encodes the distance using the parameter l. Dist can have values from
// the full range of uint32 values. To get the distance offset the actual match
// distance has to be decreased by 1. A distance offset of 0xffffffff (eos)
// indicates the end of the stream.
func (dc *distCodec) Encode(e *rangeEncoder, dist uint32, l uint32) (err error) {
	// Compute the posSlot using nlz32
	var posSlot uint32
	var _bits uint32
	if dist < startPosModel {
		posSlot = dist
	} else {
		_bits = uint32(30 - bits.LeadingZeros32(dist))
		posSlot = startPosModel - 2 + (_bits << 1)
		posSlot += (dist >> uint(_bits)) & 1
	}

	if err = dc.posSlotCodecs[lenState(l)].Encode(e, posSlot); err != nil {
		return
	}

	switch {
	case posSlot < startPosModel:
		return nil
	case posSlot < endPosModel:
		tc := &dc.posModel[posSlot-startPosModel]
		return tc.Encode(dist, e)
	}
	dic := directCodec(_bits - alignBits)
	if err = dic.Encode(e, dist>>alignBits); err != nil {
		return
	}
	return dc.alignCodec.Encode(dist, e)
}

// Decode decodes the distance offset using the parameter l. The dist value
// 0xffffffff (eos) indicates the end of the stream. Add one to the distance
// offset to get the actual match distance.
func (dc *distCodec) Decode(d *rangeDecoder, l uint32) (dist uint32, err error) {
	posSlot, err := dc.posSlotCodecs[lenState(l)].Decode(d)
	if err != nil {
		return
	}

	// posSlot equals distance
	if posSlot < startPosModel {
		return posSlot, nil
	}

	// posSlot uses the individual models
	bits := (posSlot >> 1) - 1
	dist = (2 | (posSlot & 1)) << bits
	var u uint32
	if posSlot < endPosModel {
		tc := &dc.posModel[posSlot-startPosModel]
		if u, err = tc.Decode(d); err != nil {
			return 0, err
		}
		dist += u
		return dist, nil
	}

	// posSlots use direct encoding and a single model for the four align
	// bits.
	dic := directCodec(bits - alignBits)
	if u, err = dic.Decode(d); err != nil {
		return 0, err
	}
	dist += u << alignBits
	if u, err = dc.alignCodec.Decode(d); err != nil {
		return 0, err
	}
	dist += u
	return dist, nil
}

// directCodec allows the encoding and decoding of values with a fixed number
// of bits. The number of bits must be in the range [1,32].
type directCodec byte

// Bits returns the number of bits supported by this codec.
func (dc directCodec) Bits() int {
	return int(dc)
}

// Encode uses the range encoder to encode a value with the fixed number of
// bits. The most-significant bit is encoded first.
func (dc directCodec) Encode(e *rangeEncoder, v uint32) error {
	for i := int(dc) - 1; i >= 0; i-- {
		if err := e.DirectEncodeBit(v >> uint(i)); err != nil {
			return err
		}
	}
	return nil
}

// Decode uses the range decoder to decode a value with the given number of
// given bits. The most-significant bit is decoded first.
func (dc directCodec) Decode(d *rangeDecoder) (v uint32, err error) {
	for i := int(dc) - 1; i >= 0; i-- {
		x, err := d.directDecodeBit()
		if err != nil {
			return 0, err
		}
		v = (v << 1) | x
	}
	return v, nil
}
