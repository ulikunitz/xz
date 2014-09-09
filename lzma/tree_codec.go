package lzma

type treeEncoder struct {
	probTree
}

func newTreeEncoder(bits int) *treeEncoder {
	return &treeEncoder{makeProbTree(bits)}
}

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

type treeDecoder struct {
	probTree
}

func newTreeDecoder(bits int) *treeDecoder {
	return &treeDecoder{makeProbTree(bits)}
}

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

func (t *probTree) Bits() int {
	return int(t.bits)
}
