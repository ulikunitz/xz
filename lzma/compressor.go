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

type CompressorParams struct {
	LC           int
	LP           int
	PB           int
	DictSize     int64
	ExtraBufSize int64
}

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
		return lzmaError{fmt.Sprintf("DictSize %d too large for int", p.DictSize)}
	}
	if p.ExtraBufSize < 0 {
		return negError{"ExtraBufSize", p.ExtraBufSize}
	}
	bufSize := p.DictSize + p.ExtraBufSize
	if bufSize != int64(int(bufSize)) {
		return lzmaError{"buffer size too large for int"}
	}
	return nil
}

type Compressor struct {
	properties Properties
	OpFinder   OpFinder
	state      *State
	re         *rangeEncoder
	buf        *buffer
	closed     bool
	start      int64
}

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
	d.sync()
	props := p.Properties()
	state := NewState(props, d)
	c = &Compressor{
		properties: props,
		OpFinder:   Greedy,
		state:      state,
		buf:        buf,
		re:         newRangeEncoder(lzma),
		start:      buf.top,
	}
	return c, nil
}

func (c *Compressor) Write(p []byte) (n int, err error) {
	panic("TODO")
}

func (c *Compressor) Compress(limit int64, all bool) (n int64, err error) {
	panic("TODO")
}

func (c *Compressor) Flush() error {
	panic("TODO")
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
