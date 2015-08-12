package newlzma

import "errors"

// buffer provides a circular buffer. If front equals rear the buffer is
// empty. As a consequence one byte in the data slice cannot be used to
// ensure that front != rear.
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

// Cap returns the capacity of the buffer.
func (b *buffer) Cap() int {
	return len(b.data) - 1
}

// Len provides the number of bytes buffered.
func (b *buffer) Buffered() int {
	delta := int(b.front) - int(b.rear)
	if delta < 0 {
		return len(b.data) + delta
	}
	return delta
}

// Read reads byte from the buffer into p and returns the number of
// bytes read. It never returns an error.
func (b *buffer) Read(p []byte) (n int, err error) {
	if b.rear > b.front {
		k := copy(p, b.data[b.rear:])
		b.rear += k
		if b.rear == len(b.data) {
			b.rear = 0
		}
		if k == len(p) {
			return k, nil
		}
		p = p[k:]
		n = k
	}
	k := copy(p, b.data[b.rear:b.front])
	b.rear += k
	return n + k, nil
}

// Discard skips the n next bytes to read from the buffer, returning the
// bytes discarded.
//
// If Discards skips fewer than n bytes, it returns an error.
func (b *buffer) Discard(n int) (discarded int, err error) {
	if n < 0 {
		panic("negative argument")
	}
	if b.rear > b.front {
		k := len(b.data) - int(b.rear)
		if n < k {
			b.rear += n
			return n, nil
		}
		discarded += k
		b.rear = 0
		n -= k
	}
	k := b.front - b.rear
	if n <= k {
		b.rear += n
		return discarded + n, nil
	}
	b.rear += k
	return discarded + k, errors.New("discarded less bytes then requested")
}

// Write puts data into the  buffer. If less bytes are written than
// requested an error is returned.
func (b *buffer) Write(p []byte) (n int, err error) {
	r := b.rear - 1
	if r < 0 {
		r += len(b.data)
	}
	if b.front > r {
		k := copy(b.data[b.front:], p)
		b.front += k
		if b.front == len(b.data) {
			b.front = 0
		}
		if k == len(p) {
			return k, nil
		}
		p = p[k:]
		n += k
	}
	k := copy(b.data[b.front:r], p)
	b.front += k
	n += k
	if k < len(p) {
		return n, errors.New("not all data written")
	}
	return n, nil
}
