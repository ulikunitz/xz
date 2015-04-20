package lzbase

// states defines the overall state count
const states = 12

// Value of the end of stream (EOS) marker.
const eosDist = 1<<32 - 1

// dictionary abstracts the two concrete Dictionaries away.
type dictionary interface {
	Byte(dist int64) byte
	Offset() int64
}

// state maintains the full state of the operation encoding process.
type state struct {
	Properties  Properties
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
	lc, lp, pb := s.Properties.LC(), s.Properties.LP(), s.Properties.PB()
	*s = state{
		Properties: s.Properties,
		dict:       s.dict,
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

// initState initialized a state variable.
func initState(s *state, p Properties, dict dictionary) {
	s.Properties = p
	s.dict = dict
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
func (s *state) states() (state1, state2, posState uint32) {
	state1 = s.state
	posState = uint32(s.dict.Offset()) & s.posBitMask
	state2 = (s.state << maxPosBits) | posState
	return
}

// litState computes the literal state.
func (s *state) litState() uint32 {
	prevByte := s.dict.Byte(1)
	lp, lc := uint(s.Properties.LP()), uint(s.Properties.LC())
	litState := ((uint32(s.dict.Offset())) & ((1 << lp) - 1) << lc) |
		(uint32(prevByte) >> (8 - lc))
	return litState
}

// ReaderState represents the state of the operation decoding process.
type ReaderState struct {
	state
}

// NewReaderState allocates and initializes the ReaderState.
func NewReaderState(p Properties, rd *ReaderDict) *ReaderState {
	rs := new(ReaderState)
	initState(&rs.state, p, rd)
	return rs
}

// ReaderDict returns the reader dictionary.
func (rs *ReaderState) ReaderDict() *ReaderDict {
	rd, ok := rs.dict.(*ReaderDict)
	if !ok {
		panic("ReaderState value doesn't contain a ReaderDict value")
	}
	return rd
}

// WriterState represents the state of the operation encoding process.
type WriterState struct {
	state
}

// NewWriterState allocates and initializes a WriterState value.
func NewWriterState(p Properties, wd *WriterDict) *WriterState {
	ws := new(WriterState)
	initState(&ws.state, p, wd)
	return ws
}

// WriterDict returns the writer dictionary.
func (ws *WriterState) WriterDict() *WriterDict {
	wd, ok := ws.dict.(*WriterDict)
	if !ok {
		panic("WriterState value doesn't contain a WriterDict value")
	}
	return wd
}
