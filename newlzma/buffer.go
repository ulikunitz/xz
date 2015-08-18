package newlzma

import "errors"

// buffer provides a circular buffer. If front equals rear the buffer is
// empty. As a consequence one byte in the data slice cannot be used to
// ensure that front != rear if the buffer is full.
type buffer struct {
	data  []byte
	front int
	rear  int
}

// initBuffer initializes a buffer with a given capacity. If the
// capacity is out of range an error is returned.
func initBuffer(b *buffer, capacity int) error {
	// second condition checks for overflow
	if !(0 < capacity && 0 < capacity+1) {
		return errors.New("buffer capacity out of range")
	}
	*b = buffer{data: make([]byte, capacity+1)}
	return nil
}

// Resets the buffer. The front and rear index are set to zero.
func (b *buffer) Reset() {
	b.front = 0
	b.rear = 0
}

// Cap returns the capacity of the buffer.
func (b *buffer) Cap() int {
	return len(b.data) - 1
}

// Len provides the number of bytes buffered.
func (b *buffer) Buffered() int {
	delta := b.front - b.rear
	if delta < 0 {
		delta += len(b.data)
	}
	return delta
}

func (b *buffer) Available() int {
	delta := b.rear - 1 - b.front
	if delta < 0 {
		delta += len(b.data)
	}
	return delta
}

// Read reads byte from the buffer into p and returns the number of
// bytes read. It never returns an error.
func (b *buffer) Read(p []byte) (n int, err error) {
	m := b.Buffered()
	n = len(p)
	if m < n {
		n = m
		p = p[:n]
	}
	k := copy(p, b.data[b.rear:])
	if k < n {
		copy(p[k:], b.data)
	}
	b.rear += n
	if b.rear >= len(b.data) {
		b.rear -= len(b.data)
	}
	return n, nil
}

// Discard skips the n next bytes to read from the buffer, returning the
// bytes discarded.
//
// If Discards skips fewer than n bytes, it returns an error.
func (b *buffer) Discard(n int) (discarded int, err error) {
	if n < 0 {
		panic("negative argument")
	}
	m := b.Buffered()
	if m < n {
		n = m
		err = errors.New("discarded less bytes then requested")
	}
	b.rear += n
	if b.rear >= len(b.data) {
		b.rear -= len(b.data)
	}
	return n, err
}

// Write puts data into the  buffer. If less bytes are written than
// requested an error is returned.
func (b *buffer) Write(p []byte) (n int, err error) {
	m := b.Available()
	n = len(p)
	if m < n {
		n = m
		p = p[:m]
		err = errors.New("not all data written")
	}
	k := copy(b.data[b.front:], p)
	if k < n {
		copy(b.data, p[k:])
	}
	b.front += n
	if b.front >= len(b.data) {
		b.front -= len(b.data)
	}
	return n, err
}
