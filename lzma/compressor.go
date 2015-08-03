package lzma

import (
	"errors"
	"fmt"
	"io"
)

// opLenMargin provides an upper limit for the encoding of a single
// operation. The value assumes that all range-encoded bits require
// actually two bits. A typical operations will be shorter.
const opLenMargin = 10

// OpFinder enables the support of multiple different OpFinder
// algorithms.
type OpFinder interface {
	findOps(s *State, all bool) []operation
	fmt.Stringer
}

// CompressorParams contains all the parameters for the compressor.
type CompressorParams struct {
	LC           int
	LP           int
	PB           int
	DictSize     int64
	ExtraBufSize int64
	Limit        int64
	MarkEOS      bool
}

// Properties returns the properties  as a single byte.
func (p *CompressorParams) Properties() Properties {
	props, err := NewProperties(p.LC, p.LP, p.PB)
	if err != nil {
		panic(err)
	}
	return props
}

// SetProperties sets the LC, LP and PB fields.
func (p *CompressorParams) SetProperties(props Properties) {
	p.LC, p.LP, p.PB = props.LC(), props.LP(), props.PB()
}

// Verify checks parameters for errors.
func (p *CompressorParams) Verify() error {
	if p == nil {
		return lzmaError{"parameters must be non-nil"}
	}
	if err := verifyProperties(p.LC, p.LP, p.PB); err != nil {
		return err
	}
	if !(MinDictSize <= p.DictSize && p.DictSize <= MaxDictSize) {
		return rangeError{"DictSize", p.DictSize}
	}
	if p.DictSize != int64(int(p.DictSize)) {
		return lzmaError{fmt.Sprintf("DictSize %d too large for int",
			p.DictSize)}
	}
	if p.ExtraBufSize < 0 {
		return negError{"ExtraBufSize", p.ExtraBufSize}
	}
	bufSize := p.DictSize + p.ExtraBufSize
	if bufSize != int64(int(bufSize)) {
		return lzmaError{"buffer size too large for int"}
	}
	if p.Limit < 0 {
		return negError{"Limit", p.Limit}
	}
	if 0 < p.Limit && p.Limit < 5 {
		return lzmaError{"non-zero limit must be greater or equal 5"}
	}
	return nil
}

// Compressor provides functionality to compress an uncompressed byte
// stream. The Compressor supports a limit on written bytes and is
// resettable. It is intended to be used for LZMA and LZMA2 compression
// formats.
type Compressor struct {
	properties Properties
	OpFinder   OpFinder
	state      *State
	re         *rangeEncoder
	dict       *hashDict
	closed     bool
	start      int64
	margin     int64
	markEOS    bool
}

// NewCompressor creates a compressor instance.
func NewCompressor(lzma io.Writer, p CompressorParams) (c *Compressor, err error) {
	if lzma == nil {
		return nil, errors.New("NewCompressor: argument lzma is nil")
	}
	if err = p.Verify(); err != nil {
		return nil, err
	}
	buf, err := newBuffer(p.DictSize + p.ExtraBufSize)
	if err != nil {
		return nil, err
	}
	d, err := newHashDict(buf, buf.bottom, p.DictSize)
	if err != nil {
		return nil, err
	}
	d.syncLimit()
	props := p.Properties()
	state := NewState(props, d)
	re, err := newRangeEncoderLimit(lzma, p.Limit)
	if err != nil {
		return nil, err
	}
	margin := int64(opLenMargin)
	if p.MarkEOS {
		margin += opLenMargin
	}
	c = &Compressor{
		properties: props,
		OpFinder:   Greedy,
		state:      state,
		dict:       d,
		re:         re,
		start:      d.head,
		margin:     margin,
		markEOS:    p.MarkEOS,
	}
	return c, nil
}

// Write writes bytes into the compression buffer. Note that there might
// be not enough space available in the buffer itself.
func (c *Compressor) Write(p []byte) (n int, err error) {
	if c.closed {
		return 0, errCompressorClosed
	}
	return c.dict.buf.Write(p)
}

// writeLiteral writes a literal into the operation stream
func (c *Compressor) writeLiteral(l lit) error {
	var err error
	state, state2, _ := c.state.states()
	if err = c.state.isMatch[state2].Encode(c.re, 0); err != nil {
		return err
	}
	litState := c.state.litState()
	match := c.state.dict.byteAt(int64(c.state.rep[0]) + 1)
	err = c.state.litCodec.Encode(c.re, l.b, state, match, litState)
	if err != nil {
		return err
	}
	c.state.updateStateLiteral()
	return nil
}

// iverson implements the Iverson operator as proposed by Donald Knuth in his
// book Concrete Mathematics.
func iverson(ok bool) uint32 {
	if ok {
		return 1
	}
	return 0
}

// writeMatch writes a repetition operation into the operation stream
func (c *Compressor) writeMatch(m match) error {
	var err error
	if !(minDistance <= m.distance && m.distance <= maxDistance) {
		panic(rangeError{"match distance", m.distance})
	}
	dist := uint32(m.distance - minDistance)
	if !(MinLength <= m.n && m.n <= MaxLength) &&
		!(dist == c.state.rep[0] && m.n == 1) {
		panic(rangeError{"match length", m.n})
	}
	state, state2, posState := c.state.states()
	if err = c.state.isMatch[state2].Encode(c.re, 1); err != nil {
		return err
	}
	g := 0
	for ; g < 4; g++ {
		if c.state.rep[g] == dist {
			break
		}
	}
	b := iverson(g < 4)
	if err = c.state.isRep[state].Encode(c.re, b); err != nil {
		return err
	}
	n := uint32(m.n - MinLength)
	if b == 0 {
		// simple match
		c.state.rep[3], c.state.rep[2], c.state.rep[1], c.state.rep[0] =
			c.state.rep[2], c.state.rep[1], c.state.rep[0], dist
		c.state.updateStateMatch()
		if err = c.state.lenCodec.Encode(c.re, n, posState); err != nil {
			return err
		}
		return c.state.distCodec.Encode(c.re, dist, n)
	}
	b = iverson(g != 0)
	if err = c.state.isRepG0[state].Encode(c.re, b); err != nil {
		return err
	}
	if b == 0 {
		// g == 0
		b = iverson(m.n != 1)
		if err = c.state.isRepG0Long[state2].Encode(c.re, b); err != nil {
			return err
		}
		if b == 0 {
			c.state.updateStateShortRep()
			return nil
		}
	} else {
		// g in {1,2,3}
		b = iverson(g != 1)
		if err = c.state.isRepG1[state].Encode(c.re, b); err != nil {
			return err
		}
		if b == 1 {
			// g in {2,3}
			b = iverson(g != 2)
			err = c.state.isRepG2[state].Encode(c.re, b)
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
	return c.state.repLenCodec.Encode(c.re, n, posState)
}

// discard processes an operation after it has been written into the
// compressed LZMA street by moving the dictionary head forward.
func (c *Compressor) discard(op operation) error {
	k := op.Len()
	n, err := c.dict.move(k)
	if err != nil {
		return fmt.Errorf("operation %s: move %d error %s", op, k, err)
	}
	if n < k {
		return fmt.Errorf("operation %s: move %d incomplete", op, k)
	}
	return nil
}

// writeOp writes an operation value into the stream. It checks whether there
// is still enough space available using an upper limit for the size required.
func (c *Compressor) writeOp(op operation) error {
	var err error
	switch x := op.(type) {
	case match:
		err = c.writeMatch(x)
	case lit:
		err = c.writeLiteral(x)
	}
	if err != nil {
		return err
	}
	err = c.discard(op)
	return err
}

// Compress compresses data into the buffer. If all is set the complete
// buffer will be compressed. If it is not set the last operation will
// not be written with the intention to extend a copy operation if
// additonal data becomes available.
func (c *Compressor) Compress(all bool) error {
	if c.closed {
		return errCompressorClosed
	}
	ops := c.OpFinder.findOps(c.state, all)
	for _, op := range ops {
		if c.Filled() {
			break
		}
		if err := c.writeOp(op); err != nil {
			return err
		}
	}
	c.dict.syncLimit()
	return nil
}

// Filled indicates that the underlying writer has reached the size
// limit.
func (c *Compressor) Filled() bool {
	return c.re.Available() < c.margin
}

// Close closes the current LZMA stream.
func (c *Compressor) Close() error {
	if c.closed {
		return errCompressorClosed
	}
	if c.markEOS {
		if err := c.writeMatch(eosMatch); err != nil {
			return err
		}
	}
	if err := c.re.Close(); err != nil {
		return err
	}
	c.closed = true
	return nil
}

func (c *Compressor) ResetState() {
	panic("TODO")
}

func (c *Compressor) SetProperties(p Properties) {
	panic("TODO")
}

func (c *Compressor) ResetDict(p Properties) {
	panic("TODO")
}

func (c *Compressor) SetWriter(w io.Writer) {
	panic("TODO")
}
