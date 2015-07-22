package lzma

import "io"

// opLenMargin provides an upper limit for the encoding of a single
// operation. The value assumes that all range-encoded bits require
// actually two bits. A typical operations will be shorter.
const opLenMargin = 10

type Compressor struct {
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
