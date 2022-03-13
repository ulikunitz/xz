package lzma

import (
	"fmt"

	"github.com/ulikunitz/lz"
)

// encoder supporst the LZMA encoding.
type encoder struct {
	window *lz.Window
	state  state
	pos    int64
	re     rangeEncoder
}

// byteAtEnd returns the byte with the offset i to the end of the encoding.
// Offsets outside of the window are only allowed for encoding position 0.
func (e *encoder) byteAtEnd(i int64) byte {
	c, err := e.window.ReadByteAt(e.pos - i)
	if err != nil {
		if e.pos != 0 {
			panic(err)
		}
	}
	return c
}

// writeLiteral encodes a single literal byte.
func (e *encoder) writeLiteral(c byte) error {
	state, state2, _ := e.state.states(e.pos)
	var err error
	if err = e.re.EncodeBit(0, &e.state.s2[state2].isMatch); err != nil {
		return err
	}
	litState := e.state.litState(e.byteAtEnd(1), e.pos)
	match := e.byteAtEnd(int64(e.state.rep[0]) + 1)
	err = e.state.litCodec.Encode(&e.re, c, state, match, litState)
	if err != nil {
		return err
	}
	e.state.updateStateLiteral()
	e.pos++
	return nil
}

// iverson returns 1 for true and 0 for false. It is intended to be inlined.
func iverson(f bool) uint32 {
	if f {
		return 1
	}
	return 0
}

// writeMatch writes a match. The argument dist equals offset - 1.
func (e *encoder) writeMatch(dist, matchLen uint32) error {
	var err error

	if !(minMatchLen <= matchLen && matchLen <= maxMatchLen) &&
		!(dist == e.state.rep[0] && matchLen == 1) {
		return fmt.Errorf(
			"match length %d out of range; dist %d rep[0] %d",
			matchLen, dist, e.state.rep[0])
	}
	state, state2, posState := e.state.states(e.pos)
	if err = e.re.EncodeBit(1, &e.state.s2[state2].isMatch); err != nil {
		return err
	}
	g := 0
	for ; g < 4; g++ {
		if e.state.rep[g] == dist {
			break
		}
	}
	b := iverson(g < 4)
	if err = e.re.EncodeBit(b, &e.state.s1[state].isRep); err != nil {
		return err
	}
	n := matchLen - minMatchLen
	if b == 0 {
		// simple match
		e.state.rep[3], e.state.rep[2], e.state.rep[1], e.state.rep[0] =
			e.state.rep[2], e.state.rep[1], e.state.rep[0], dist
		e.state.updateStateMatch()
		err = e.state.lenCodec.Encode(&e.re, n, posState)
		if err != nil {
			return err
		}
		if err = e.state.distCodec.Encode(&e.re, dist, n); err != nil {
			return err
		}
		e.pos += int64(matchLen)
		return nil
	}
	b = iverson(g != 0)
	if err = e.re.EncodeBit(b, &e.state.s1[state].isRepG0); err != nil {
		return err
	}
	if b == 0 {
		// g == 0
		b = uint32(iverson(matchLen != 1))
		err = e.re.EncodeBit(b, &e.state.s2[state2].isRepG0Long)
		if err != nil {
			return err
		}
		if b == 0 {
			e.state.updateStateShortRep()
			e.pos++
			return nil
		}
	} else {
		// g in {1,2,3}
		b = uint32(iverson(g != 1))
		err = e.re.EncodeBit(b, &e.state.s1[state].isRepG1)
		if err != nil {
			return err
		}
		if b == 1 {
			// g in {2,3}
			b = iverson(g != 2)
			err = e.re.EncodeBit(b, &e.state.s1[state].isRepG2)
			if err != nil {
				return err
			}
			if b == 1 {
				e.state.rep[3] = e.state.rep[2]
			}
			e.state.rep[2] = e.state.rep[1]
		}
		e.state.rep[1] = e.state.rep[0]
		e.state.rep[0] = dist
	}
	e.state.updateStateRep()
	if err = e.state.repLenCodec.Encode(&e.re, n, posState); err != nil {
		return err
	}
	e.pos += int64(matchLen)
	return nil
}
