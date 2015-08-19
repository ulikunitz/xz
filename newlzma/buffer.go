package newlzma

import "errors"

// buffer provides a circular buffer. If the front index equals the rear
// index the buffer is empty. As a consequence front cannot be equal
// rear for a full buffer. So a full buffer has a length that is one
// byte less the the length of the data slice.
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

// Buffered returns the number of bytes buffered.
func (b *buffer) Buffered() int {
	delta := b.front - b.rear
	if delta < 0 {
		delta += len(b.data)
	}
	return delta
}

// Available returns the number of bytes available for writing.
func (b *buffer) Available() int {
	delta := b.rear - 1 - b.front
	if delta < 0 {
		delta += len(b.data)
	}
	return delta
}

// addIndex adds a non-negative integer to the index i and returns the
// resulting index. The function takes care of wrapping the index as
// well as potential overflow situations.
func (b *buffer) addIndex(i int, n int) int {
	// subtraction of len(b.data) prevents overflow
	i += n - len(b.data)
	if i < 0 {
		i += len(b.data)
	}
	return i
}

// Read reads byte from the buffer into p and returns the number of
// bytes read. The functions never returns an error.
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
	b.rear = b.addIndex(b.rear, n)
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
	b.rear = b.addIndex(b.rear, n)
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
	b.front = b.addIndex(b.front, n)
	return n, err
}

// errNoSpace indicates that free space in the buffer is not sufficient.
var errNoSpace = errors.New("not enough space in buffer")

// WriteByte writes a single byte into the buffer. An error is returned
// if there is not enough space.
func (b *buffer) WriteByte(c byte) error {
	if b.Available() < 1 {
		return errNoSpace
	}
	b.data[b.front] = c
	b.front = b.addIndex(b.front, 1)
	return nil
}
