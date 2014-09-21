package lzma

// maxPosBits defines the number of bits of the position value that are used to
// to compute the posState value. The value is used to selet the tree codec
// for length encoding and decoding.
const maxPosBits = 4

// minLength and maxLength give the minimum and maximum values for encoding and
// decoding length values.
const (
	minLength = 2
	maxLength = minLength + 16 + 256 - 1
)

// lengthCodec support the encoding of the length value.
type lengthCodec struct {
	choice [2]prob
	low    [1 << maxPosBits]treeCodec
	mid    [1 << maxPosBits]treeCodec
	high   treeCodec
}

// newLengthCodec() creates and initializes a new length codec.
func newLengthCodec() *lengthCodec {
	lc := new(lengthCodec)
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
	return lc
}

// Encode encodes a length value.
func (lc *lengthCodec) Encode(l uint32, e *rangeEncoder, posState uint32,
) (err error) {
	l -= 2
	if l < 8 {
		if err = lc.choice[0].Encode(0, e); err != nil {
			return
		}
		return lc.low[posState].Encode(l, e)
	}
	if err = lc.choice[0].Encode(1, e); err != nil {
		return
	}
	if l < 16 {
		if err = lc.choice[1].Encode(0, e); err != nil {
			return
		}
		return lc.mid[posState].Encode(l-8, e)
	}
	if err = lc.choice[1].Encode(1, e); err != nil {
		return
	}
	return lc.high.Encode(l-16, e)
}

// Decode decodes a length value.
func (lc *lengthCodec) Decode(d *rangeDecoder, posState uint32,
) (l uint32, err error) {
	var b uint32
	if b, err = lc.choice[0].Decode(d); err != nil {
		return
	}
	if b == 0 {
		l, err = lc.low[posState].Decode(d)
		l += 2
		return
	}
	if b, err = lc.choice[1].Decode(d); err != nil {
		return
	}
	if b == 0 {
		l, err = lc.mid[posState].Decode(d)
		l += 2 + 8
		return
	}
	l, err = lc.high.Decode(d)
	l += 2 + 16
	return
}