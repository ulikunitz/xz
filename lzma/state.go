// Copyright 2015 Ulrich Kunitz. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package lzma

// states defines the overall state count
const states = 12

// state maintains the full state of the operation encoding process.
type state struct {
	properties  Properties
	state       uint32
	posBitMask  uint32
	isMatch     [states << maxPosBits]prob
	isRep       [states]prob
	isRepG0     [states]prob
	isRepG1     [states]prob
	isRepG2     [states]prob
	isRepG0Long [states << maxPosBits]prob
	rep         [4]uint32
	litCodec    literalCodec
	lenCodec    lengthCodec
	repLenCodec lengthCodec
	distCodec   distCodec
}

// initProbSlice initializes a slice of probabilities.
func initProbSlice(p []prob) {
	for i := range p {
		p[i] = probInit
	}
}

// Reset sets all state information to the original values.
func (s *state) Reset() {
	lc, lp, pb := s.properties.LC(), s.properties.LP(), s.properties.PB()
	*s = state{
		properties: s.properties,
		// dict:       s.dict,
		posBitMask: (uint32(1) << uint(pb)) - 1,
	}
	initProbSlice(s.isMatch[:])
	initProbSlice(s.isRep[:])
	initProbSlice(s.isRepG0[:])
	initProbSlice(s.isRepG1[:])
	initProbSlice(s.isRepG2[:])
	initProbSlice(s.isRepG0Long[:])
	s.litCodec.init(lc, lp)
	s.lenCodec.init()
	s.repLenCodec.init()
	s.distCodec.init()
}

// initState initializes the state.
func initState(s *state, p Properties) {
	*s = state{properties: p}
	s.Reset()
}

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
	lp, lc := uint(s.properties.LP()), uint(s.properties.LC())
	litState := ((uint32(dictHead) & ((1 << lp) - 1)) << lc) |
		(uint32(prev) >> (8 - lc))
	return litState
}
