package lzma

// treeEncoder provides an fixed-bit-size encoder. The encoder uses a
// probability tree for the bits. The tree starts with the most-significant
// bit.
type treeEncoder struct {
	probTree
}

// makeTreeEncoder makes a tree encoder. It might panic if the bits argument is
// not inside the range [1,32].
func makeTreeEncoder(bits int) treeEncoder {
	return treeEncoder{makeProbTree(bits)}
}

// Encode uses the range encoder to encode a fixed-bit-size value.
func (te *treeEncoder) Encode(v uint32, e *rangeEncoder) (err error) {
	m := uint32(1)
	for i := int(te.bits) - 1; i >= 0; i-- {
		b := (v >> uint(i)) & 1
		if err := e.EncodeBit(b, &te.probs[m]); err != nil {
			return err
		}
		m = (m << 1) | b
	}
	return nil
}

// treeDecoder provides a fixed-bit-size decoder. The decoder uses a
// probability tree for the bits. The tree starts with the most-significant
// bit.
type treeDecoder struct {
	probTree
}

// makeTreeDecoder makes a tree decoder.
func makeTreeDecoder(bits int) treeDecoder {
	return treeDecoder{makeProbTree(bits)}
}

// Decodes uses the range decoder to decode a fixed-bit-size value. Errors may
// be caused by the range decoder.
func (td *treeDecoder) Decode(d *rangeDecoder) (v uint32, err error) {
	m := uint32(1)
	for j := 0; j < int(td.bits); j++ {
		b, err := d.DecodeBit(&td.probs[m])
		if err != nil {
			return 0, err
		}
		m = (m << 1) | b
	}
	return m - (1 << uint(td.bits)), nil
}

// treeReverseEncoder provides a fixed-bit-size encoder. The encoder uses a
// probability tree for the bits. The tree starts with the least-significant
// bit.
type treeReverseEncoder struct {
	probTree
}

// makeTreeReverseEncoder creates an encoder. The function will panic if bits
// is outside [1,32].
func makeTreeReverseEncoder(bits int) treeReverseEncoder {
	return treeReverseEncoder{makeProbTree(bits)}
}

// Encoder uses range encoder to encode a fixed-bit-size value. The range
// encoder may cause errors.
func (te *treeReverseEncoder) Encode(v uint32, e *rangeEncoder) (err error) {
	m := uint32(1)
	for i := uint(0); i < uint(te.bits); i++ {
		b := (v >> uint(i)) & 1
		if err := e.EncodeBit(b, &te.probs[m]); err != nil {
			return err
		}
		m = (m << 1) | b
	}
	return nil
}

// treeReverseDecoder decodes fixed-bit-size values. The decoder uses a
// probability tree that starts with the least-significant bit.
type treeReverseDecoder struct {
	probTree
}

// makeTreeReverseEncoder creates a treeReverseDecoder. The function might
// panic if bits is outside [1,32].
func makeTreeReverseDecoder(bits int) treeReverseDecoder {
	return treeReverseDecoder{makeProbTree(bits)}
}

// Decodes uses the range decoder to decode a fixed-bit-size value. Errors
// returned by the range decoder will be returned.
func (td *treeReverseDecoder) Decode(d *rangeDecoder) (v uint32, err error) {
	m := uint32(1)
	for j := uint(0); j < uint(td.bits); j++ {
		b, err := d.DecodeBit(&td.probs[m])
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
