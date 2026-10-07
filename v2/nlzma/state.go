package nlzma

import (
	"errors"
	"fmt"
)

// Properties the parameters for the LZMA encoding.
type Properties struct {
	// LC TODO
	LC int
	// LP TODO
	LP int
	// PB provides the number of least significant bits used for the
	// position.
	PB int
}

func (p Properties) byte() byte {
	return (byte)((p.PB*5+p.LP)*9 + p.LC)
}

func (p *Properties) fromByte(b byte) error {
	p.LC = int(b % 9)
	b /= 9
	p.LP = int(b % 5)
	b /= 5
	p.PB = int(b)
	if p.PB > 4 {
		return errors.New("lzma: invalid properties byte")
	}
	return nil
}

// verify verifies the correctness of the properties. It doesn't check the LZMA2
// condition that LC + LP <= 4.
func (p Properties) verify() error {
	if !(0 <= p.LC && p.LC <= 8) {
		return fmt.Errorf("lzma: LC out of range 0..8")
	}
	if !(0 <= p.LP && p.LP <= 4) {
		return fmt.Errorf("lzma: LP out of range 0..4")
	}
	if !(0 <= p.PB && p.PB <= 4) {
		return fmt.Errorf("lzma: PB out of range 0..4")
	}
	return nil
}

// moveBits defines the number of bits used for the move operation.
const moveBits = 5

// probBits defines the number of bits used to represent the probability.
const probBits = 11

type prob uint16

// probInit is the initial probability value set to 0.5.
const probInit prob = 1 << (probBits - 1)

func incProb(p prob) prob {
	return p + ((1<<probBits)-p)>>moveBits
}

func decProb(p prob) prob {
	return p - (p >> moveBits)
}

// bound computes the new bound for a given range using the probability value.
func (p prob) bound(r uint32) uint32 {
	return (r >> probBits) * uint32(p)
}

const states = 12

type state struct {
	_probs      []prob
	s1          []prob
	s2          []prob
	litCodec    literalCodec
	lenCodec    lengthCodec
	repLenCodec lengthCodec
	distCodec   distCodec
	Properties
	rep        [4]uint32
	state      uint32
	posBitMask uint32
}

const (
	s1IsRep0 = iota
	s1IsRepG0
	s1IsRepG1
	s1IsRepG2
)

func s1(s *state, state int, rep int) *prob {
	return &s.s1[4*state+rep]
}

const (
	s2IsMatch = iota
	s2IsRepG0Long
)

func s2(s *state, state int, rep int) *prob {
	return &s.s2[4*state+rep]
}

const (
	s1ProbLen = 4 * states
)

func s2ProbLen(pb int) int {
	return 2 * (states << pb)
}

func litCodecProbLen(lc, lp int) int {
	return 0x300 << (lc + lp)
}

func treeProbLen(bits int) int {
	return 1 << bits
}

func lengthCodecProbLen(pb int) int {
	return 2 + 2*(1<<pb)*treeProbLen(3) + treeProbLen(8)
}

// Constants used by the distance codec.
const (
	// minimum supported distance
	minDistance = 1

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

func distCodecProbLen() int {
	n := lenStates * treeProbLen(posSlotBits)
	for posSlot := startPosModel; posSlot < endPosModel; posSlot++ {
		bits := (posSlot >> 1) - 1
		n += treeProbLen(bits)
	}
	n += treeProbLen(alignBits)
	return n
}

func stateProbLen(p Properties) int {
	n := s1ProbLen
	n += s2ProbLen(p.PB)
	n += litCodecProbLen(p.LC, p.LP)
	n += 2 * lengthCodecProbLen(p.PB)
	n += distCodecProbLen()
	return n
}

func (s *state) init(props Properties) {
	n := stateProbLen(props)
	s._probs = make([]prob, n)
	for i := range s._probs {
		s._probs[i] = probInit
	}
	p := s._probs
	s.s1 = p[:s1ProbLen]
	p = p[s1ProbLen:]

	k := s2ProbLen(props.PB)
	s.s2 = p[:k]
	p = p[k:]

	p = s.litCodec.init(props.LC, props.LP, p)
	p = s.lenCodec.init(props.PB, p)
	p = s.repLenCodec.init(props.PB, p)
	p = s.distCodec.init(p)
	if len(p) != 0 {
		panic(fmt.Sprintf(
			"lzma: state.init() - leftover probabilities: %d",
			len(p)))
	}

	s.Properties = props
	s.rep = [4]uint32{}
	s.state = 0
	s.posBitMask = (1 << props.PB) - 1
}

func (s *state) clone(src *state) {
	p := s._probs
	*s = *src

	if len(p) != len(src._probs) {
		p = make([]prob, len(src._probs))
	}
	s._probs = p
	copy(p, src._probs)

	s.s1 = p[:s1ProbLen]
	p = p[s1ProbLen:]

	s.s2 = p[:len(src.s2)]
	p = p[len(src.s2):]

	p = s.litCodec.clone(&src.litCodec, p)
	p = s.lenCodec.clone(&src.lenCodec, p)
	p = s.repLenCodec.clone(&src.repLenCodec, p)
	p = s.distCodec.clone(&src.distCodec, p)

	if len(p) != 0 {
		panic(fmt.Sprintf(
			"lzma: state.clone() - leftover probabilities: %d",
			len(p)))
	}
}

type literalCodec struct {
	probs []prob
}

func (lc *literalCodec) init(lcLC, lcLP int, p []prob) []prob {
	n := litCodecProbLen(lcLC, lcLP)
	lc.probs = p[:n]
	return p[n:]
}

func (lc *literalCodec) clone(src *literalCodec, p []prob) []prob {
	lc.probs = p[:len(src.probs)]
	return p[len(lc.probs):]
}

type lengthCodec struct {
	choice [2]prob
	low    []treeCodec
	mid    []treeCodec
	high   treeCodec
}

func (lc *lengthCodec) init(pb int, p []prob) []prob {
	lc.choice[0] = p[0]
	lc.choice[1] = p[1]
	p = p[2:]

	lc.low = make([]treeCodec, 1<<pb)
	for i := range lc.low {
		p = lc.low[i].init(3, p)
	}
	lc.mid = make([]treeCodec, 1<<pb)
	for i := range lc.mid {
		p = lc.mid[i].init(3, p)
	}
	p = lc.high.init(8, p)
	return p
}

func (lc *lengthCodec) clone(src *lengthCodec, p []prob) []prob {
	lc.choice[0] = p[0]
	lc.choice[1] = p[1]
	p = p[2:]

	if len(lc.low) != len(src.low) {
		lc.low = make([]treeCodec, len(src.low))
	}
	for i := range src.low {
		p = lc.low[i].clone(&src.low[i], p)
	}

	if len(lc.mid) != len(src.mid) {
		lc.mid = make([]treeCodec, len(src.mid))
	}
	for i := range lc.mid {
		p = lc.mid[i].clone(&src.mid[i], p)
	}
	p = lc.high.clone(&src.high, p)
	return p
}

type distCodec struct {
	posSlotCodecs [lenStates]treeCodec
	posModel      [endPosModel - startPosModel]treeReverseCodec
	alignCodec    treeReverseCodec
}

func (dc *distCodec) clone(src *distCodec, p []prob) []prob {
	for i := range lenStates {
		p = dc.posSlotCodecs[i].clone(&src.posSlotCodecs[i], p)
	}
	for i := range endPosModel - startPosModel {
		p = dc.posModel[i].clone(&src.posModel[i], p)
	}
	p = dc.alignCodec.clone(&src.alignCodec, p)
	return p
}

func (dc *distCodec) init(p []prob) []prob {
	for i := range dc.posSlotCodecs {
		p = dc.posSlotCodecs[i].init(posSlotBits, p)
	}
	for i := range dc.posModel {
		bits := ((startPosModel + i) >> 1) - 1
		p = dc.posModel[i].init(bits, p)
	}
	p = dc.alignCodec.init(alignBits, p)
	return p
}

type treeCodec struct {
	probTree
}

func (t *treeCodec) clone(src *treeCodec, p []prob) []prob {
	return t.probTree.clone(&src.probTree, p)
}

func (t *treeCodec) init(pb int, p []prob) []prob {
	return t.probTree.init(pb, p)
}

type treeReverseCodec struct {
	probTree
}

func (t *treeReverseCodec) clone(src *treeReverseCodec, p []prob) []prob {
	return t.probTree.clone(&src.probTree, p)
}

func (t *treeReverseCodec) init(pb int, p []prob) []prob {
	return t.probTree.init(pb, p)
}

type probTree struct {
	probs []prob
	bits  byte
}

func (t *probTree) clone(src *probTree, p []prob) []prob {
	t.bits = src.bits
	t.probs = p[:len(src.probs)]
	return p[len(t.probs):]
}

func (t *probTree) init(bits int, p []prob) []prob {
	t.bits = byte(bits)
	k := 1 << bits
	t.probs = p[:k]
	return p[k:]
}
