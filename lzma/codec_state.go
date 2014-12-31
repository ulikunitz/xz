package lzma

// states defines the overall state count
const states = 12

type codecState struct {
	properties Properties
	// length to unpack; NoUnpackLen requires an EOS marker
	unpackLen   uint64
	currentLen  uint64
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

func newCodecState(p *Properties, unpackLen uint64) (s *codecState, err error) {
	if err = verifyProperties(p); err != nil {
		return nil, err
	}
	s = &codecState{properties: *p, unpackLen: unpackLen}
	s.posBitMask = (uint32(1) << uint(s.properties.PB)) - 1
	initProbSlice(s.isMatch[:])
	initProbSlice(s.isRep[:])
	initProbSlice(s.isRepG0[:])
	initProbSlice(s.isRepG1[:])
	initProbSlice(s.isRepG2[:])
	initProbSlice(s.isRepG0Long[:])
	s.litCodec = newLiteralCodec(s.properties.LC, s.properties.LP)
	s.lenCodec = newLengthCodec()
	s.repLenCodec = newLengthCodec()
	s.distCodec = newDistCodec()
	return s, nil
}

func (s *codecState) Properties() Properties {
	return s.properties
}
