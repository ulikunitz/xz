package lzma

import "io"

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
