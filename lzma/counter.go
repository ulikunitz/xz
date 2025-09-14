package lzma

import (
	"fmt"

	"github.com/ulikunitz/lz"
)

type counter struct {
	window lz.Parser
	state  state
	rc     rangeBitCounter
	pos    int64
}

func (c *counter) bits() int64 {
	return c.rc.bits()
}

func (c *counter) fromEncoder(e *encoder) {
	*c = counter{
		window: e.window,
		pos:    e.pos,
	}
	c.state.deepCopy(&e.state)
	c.rc.fromRangeEncoder(&e.re)
}

func (c *counter) copy(src *counter) {
	*c = counter{
		window: src.window,
		rc:     src.rc,
		pos:    src.pos,
	}
	c.state.deepCopy(&src.state)
}

func (c *counter) byteAtEnd(i int64) byte {
	b, err := c.window.ByteAt(c.pos - i)
	if err != nil {
		if c.pos != 0 {
			panic(err)
		}
	}
	return b
}

func (c *counter) writeLiteral(b byte) error {
	state, state2, _ := c.state.states(c.pos)
	var err error
	if err = c.rc.encodeBit(0, &c.state.s2[state2].isMatch); err != nil {
		return err
	}
	litState := c.state.litState(c.byteAtEnd(1), c.pos)
	match := c.byteAtEnd(int64(c.state.rep[0]) + 1)
	err = c.state.litCodec.Encode(&c.rc, b, state, match, litState)
	if err != nil {
		return err
	}
	c.state.updateStateLiteral()
	c.pos++
	return nil
}

// writeMatch writes a match. The argument dist equals offset - 1.
func (c *counter) writeMatch(dist, matchLen uint32) error {
	var err error

	if !(minMatchLen <= matchLen && matchLen <= maxMatchLen) &&
		!(dist == c.state.rep[0] && matchLen == 1) {
		return fmt.Errorf(
			"match length %d out of range; dist %d rep[0] %d",
			matchLen, dist, c.state.rep[0])
	}
	state, state2, posState := c.state.states(c.pos)
	if err = c.rc.encodeBit(1, &c.state.s2[state2].isMatch); err != nil {
		return err
	}
	g := 0
	for ; g < 4; g++ {
		if c.state.rep[g] == dist {
			break
		}
	}
	b := iverson(g < 4)
	if err = c.rc.encodeBit(b, &c.state.s1[state].isRep); err != nil {
		return err
	}
	n := matchLen - minMatchLen
	if b == 0 {
		// simple match
		c.state.rep[3], c.state.rep[2], c.state.rep[1], c.state.rep[0] =
			c.state.rep[2], c.state.rep[1], c.state.rep[0], dist
		c.state.updateStateMatch()
		err = c.state.lenCodec.Encode(&c.rc, n, posState)
		if err != nil {
			return err
		}
		if err = c.state.distCodec.Encode(&c.rc, dist, n); err != nil {
			return err
		}
		c.pos += int64(matchLen)
		return nil
	}
	b = iverson(g != 0)
	if err = c.rc.encodeBit(b, &c.state.s1[state].isRepG0); err != nil {
		return err
	}
	if b == 0 {
		// g == 0
		b = uint32(iverson(matchLen != 1))
		err = c.rc.encodeBit(b, &c.state.s2[state2].isRepG0Long)
		if err != nil {
			return err
		}
		if b == 0 {
			c.state.updateStateShortRep()
			c.pos++
			return nil
		}
	} else {
		// g in {1,2,3}
		b = uint32(iverson(g != 1))
		err = c.rc.encodeBit(b, &c.state.s1[state].isRepG1)
		if err != nil {
			return err
		}
		if b == 1 {
			// g in {2,3}
			b = iverson(g != 2)
			err = c.rc.encodeBit(b, &c.state.s1[state].isRepG2)
			if err != nil {
				return err
			}
			if b == 1 {
				c.state.rep[3] = c.state.rep[2]
			}
			c.state.rep[2] = c.state.rep[1]
		}
		c.state.rep[1] = c.state.rep[0]
		c.state.rep[0] = dist
	}
	c.state.updateStateRep()
	if err = c.state.repLenCodec.Encode(&c.rc, n, posState); err != nil {
		return err
	}
	c.pos += int64(matchLen)
	return nil
}
