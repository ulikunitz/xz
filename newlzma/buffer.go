package newlzma

import "errors"

// computeMaxBufferCapacity returns the maximum buffer capacity support by the
// platform. On 64-bit platforms the maximum buffer capacity is 2^32-1
// and on 32-bit platforms 2^31-1. The 64-bit platform limit is given by
// the LZMA specification and the 32-bit limit by the limitations of the
// platform.
func computeMaxBufferCapacity() int64 {
	const c = 1<<32 - 1
	if int(c) < 0 {
		return 1<<31 - 2
	}
	return c
}

// maxBufferCapacity provides the maximum buffer capacity supported on
// the platform. Note that no check is done for the availability of the
// memory. On 32-bit platforms it should be 2^31-2 and on 64-bit platforms
// 2^32-1.
var maxBufferCapacity = computeMaxBufferCapacity()

// buffer provides a circular buffer with at most 2^32-1 bytes capacity.
// We use the single free byte mechanism to distinguish between empty
// and full buffer.
type buffer struct {
	data  []byte
	front uint32
	rear  uint32
}

// initBuffer initializes a buffer with a given capacity. If the
// capacity is out of range an error is returned.
func initBuffer(b *buffer, capacity int) error {
	if !(0 < capacity && int64(capacity) <= maxBufferCapacity) {
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

// Available provides the number of bytes available for writing.
func (b *buffer) Available() int {
	delta := int(b.rear) - int(b.front) - 1
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
		b.rear += uint32(k)
		if int(b.rear) == len(b.data) {
			b.rear = 0
		}
		if k == len(p) {
			return k, nil
		}
		p = p[k:]
		n = k
	}
	k := copy(p, b.data[b.rear:b.front])
	b.rear += uint32(k)
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
			b.rear += uint32(n)
			return n, nil
		}
		discarded += k
		b.rear = 0
		n -= k
	}
	k := int(b.front) - int(b.rear)
	if n <= k {
		b.rear += uint32(n)
		return discarded + n, nil
	}
	b.rear += uint32(k)
	return discarded + k, errors.New("discarded less bytes then requested")
}

// Write puts data into the  buffer. If less bytes are written than
// requested an error is returned.
func (b *buffer) Write(p []byte) (n int, err error) {
	r := int(b.rear) - 1
	if r < 0 {
		r += len(b.data)
	}
	if int(b.front) > r {
		k := copy(b.data[b.front:], p)
		b.front += uint32(k)
		if int(b.front) == len(b.data) {
			b.front = 0
		}
		if k == len(p) {
			return k, nil
		}
		p = p[k:]
		n += k
	}
	k := copy(b.data[b.front:r], p)
	b.front += uint32(k)
	n += k
	if k < len(p) {
		return n, errors.New("not all data written")
	}
	return n, nil
}
