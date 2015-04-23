package lzbase

import "io"

// WriterCounter is intended to wrap a wrapper and count the number of bytes
// written to the writer.
type WriterCounter struct {
	W io.Writer
	N int64
}

// Write calls the Write method of the wrapped writer and adds the written
// bytes to the N field.
func (c *WriterCounter) Write(p []byte) (n int, err error) {
	n, err = c.W.Write(p)
	c.N += int64(n)
	return
}
