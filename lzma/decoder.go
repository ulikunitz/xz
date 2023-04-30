package lzma

import "github.com/ulikunitz/lz"

// decoder supports the decoding of sequences.
type decoder struct {
	dict  lz.DecBuffer
	state state
	rd    rangeDecoder
}

// readSeq reads a single sequence. We are encoding a little bit differently
// than normal, because each seq is either a one-byte literal (LitLen=1, AUX has
// the byte) or a match (MatchLen and Offset non-zero).
func (d *decoder) readSeq() (seq lz.Seq, err error) {
	state, state2, posState := d.state.states(d.dict.Off)

	s2 := &d.state.s2[state2]
	b, err := d.rd.decodeBit(&s2.isMatch)
	if err != nil {
		return lz.Seq{}, err
	}
	if b == 0 {
		// literal
		litState := d.state.litState(d.dict.ByteAtEnd(1), d.dict.Off)
		match := d.dict.ByteAtEnd(int(d.state.rep[0]) + 1)
		s, err := d.state.litCodec.Decode(&d.rd, d.state.state, match,
			litState)
		if err != nil {
			return lz.Seq{}, err
		}
		d.state.updateStateLiteral()
		return lz.Seq{LitLen: 1, Aux: uint32(s)}, err
	}

	s1 := &d.state.s1[state]
	b, err = d.rd.decodeBit(&s1.isRep)
	if err != nil {
		return lz.Seq{}, err
	}
	if b == 0 {
		// simple match
		d.state.rep[3], d.state.rep[2], d.state.rep[1] =
			d.state.rep[2], d.state.rep[1], d.state.rep[0]

		d.state.updateStateMatch()
		// The length decoder returns the length offset.
		n, err := d.state.lenCodec.Decode(&d.rd, posState)
		if err != nil {
			return lz.Seq{}, err
		}
		// The dist decoder returns the distance offset. The actual
		// distance is 1 higher.
		d.state.rep[0], err = d.state.distCodec.Decode(&d.rd, n)
		if err != nil {
			return lz.Seq{}, err
		}
		if d.state.rep[0] == eosDist {
			return lz.Seq{}, errEOS
		}
		return lz.Seq{MatchLen: n + minMatchLen,
			Offset: d.state.rep[0] + minDistance}, nil
	}
	b, err = d.rd.decodeBit(&s1.isRepG0)
	if err != nil {
		return lz.Seq{}, err
	}
	dist := d.state.rep[0]
	if b == 0 {
		// rep match 0
		b, err = d.rd.decodeBit(&s2.isRepG0Long)
		if err != nil {
			return lz.Seq{}, err
		}
		if b == 0 {
			d.state.updateStateShortRep()
			return lz.Seq{MatchLen: 1, Offset: dist + minDistance},
				nil
		}
	} else {
		b, err = d.rd.decodeBit(&s1.isRepG1)
		if err != nil {
			return lz.Seq{}, err
		}
		if b == 0 {
			dist = d.state.rep[1]
		} else {
			b, err = d.rd.decodeBit(&s1.isRepG2)
			if err != nil {
				return lz.Seq{}, err
			}
			if b == 0 {
				dist = d.state.rep[2]
			} else {
				dist = d.state.rep[3]
				d.state.rep[3] = d.state.rep[2]
			}
			d.state.rep[2] = d.state.rep[1]
		}
		d.state.rep[1] = d.state.rep[0]
		d.state.rep[0] = dist
	}
	n, err := d.state.repLenCodec.Decode(&d.rd, posState)
	if err != nil {
		return lz.Seq{}, err
	}
	d.state.updateStateRep()
	return lz.Seq{MatchLen: n + minMatchLen, Offset: dist + minDistance},
		nil
}
