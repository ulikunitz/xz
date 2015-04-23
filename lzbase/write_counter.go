package lzbase

import "io"

// WriteCounter wraps a writer and counts all bytes written to it.
type WriteCounter struct {
	W io.Writer
	N int64
}

// Write writes the bytes to the wrapped writer and adds the bytes written to
// the field N.
func (c *WriteCounter) Write(p []byte) (n int, err error) {
	n, err = c.W.Write(p)
	c.N += int64(n)
	return
}
