package lzbase

// maxPosBits defines the number of bits of the position value that are used to
// to compute the posState value. The value is used to select the tree codec
// for length encoding and decoding.
const maxPosBits = 4

// MinLength and MaxLength give the minimum and maximum values for encoding and
// decoding length values. MinLength gives also the base for the encoded length
// values.
const (
	MinLength = 2
	MaxLength = MinLength + 16 + 256 - 1
)

// lengthCodec support the encoding of the length value.
type lengthCodec struct {
	choice [2]prob
	low    [1 << maxPosBits]treeCodec
	mid    [1 << maxPosBits]treeCodec
	high   treeCodec
}

// newLengthCodec() creates and initializes a new length codec.
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
// subtracting MinLength (2) from the actual length.
//
//   l = length - MinLength
//
func (lc *lengthCodec) Encode(e *rangeEncoder, l uint32, posState uint32,
) (err error) {
	debug.Printf("LEN %d", posState)
	defer debug.Printf("LEN %3d %2d\n", l, posState)
	if l > MaxLength-MinLength {
		return newError("length out of range")
	}
	if l < 8 {
		if err = lc.choice[0].Encode(e, 0); err != nil {
			return
		}
		return lc.low[posState].Encode(e, l)
	}
	if err = lc.choice[0].Encode(e, 1); err != nil {
		return
	}
	if l < 16 {
		if err = lc.choice[1].Encode(e, 0); err != nil {
			return
		}
		return lc.mid[posState].Encode(e, l-8)
	}
	if err = lc.choice[1].Encode(e, 1); err != nil {
		return
	}
	if err = lc.high.Encode(e, l-16); err != nil {
		return
	}
	return nil
}

// Decode reads the length offset. Add MinLength to compute the actual length
// to the length offset l.
func (lc *lengthCodec) Decode(d *rangeDecoder, posState uint32,
) (l uint32, err error) {
	debug.Printf("LEN %d", posState)
	defer func() {
		debug.Printf("LEN %3d %2d\n", l, posState)
	}()
	var b uint32
	if b, err = lc.choice[0].Decode(d); err != nil {
		return
	}
	if b == 0 {
		l, err = lc.low[posState].Decode(d)
		return
	}
	if b, err = lc.choice[1].Decode(d); err != nil {
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
