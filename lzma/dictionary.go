package lzma

import "errors"

// dictionary represents the dictionary for the LZ decompression. It is a ring
// buffer for decoding literals and matches as well as output. The readLen
// gives the number of bytes that can actually be read and full indicates that
// the dictionary is full.
type dictionary struct {
	buffer  []byte
	readLen int
	w       int
	full    bool
}

// reset clears the dictionary. It doesn't change the size of the dictionary.
func (d *dictionary) reset() {
	d.readLen = 0
	d.w = 0
	d.full = false
}

// init initalizes the dictionary to the given size
func (d *dictionary) init(size int) {
	d.buffer = make([]byte, size)
	d.reset()
}

// writeLen returns the number of bytes that can be written into the
// dictionary.
func (d *dictionary) writeLen() int {
	return len(d.buffer) - d.readLen
}

// maxDistance returns the maximum distance that is currently supported.
func (d *dictionary) maxDistance() int {
	if d.full {
		return len(d.buffer)
	}
	return d.w
}

// errDictionaryOverflow indicates that the dictionary will overflow for the
// current write.
var errDictionaryOverflow = errors.New("dictionary will overflow")

// errDistOutOfRange indicates that the distance is out of range.
var errDistOutOfRange = errors.New("distance out of range")

// errLengthOutOfRange indicates that the length is either too large or too
// small.
var errLengthOutOfRange = errors.New("length out of range")

// put puts a byte into the buffer. Note that it doesn't check whether writeLen
// is larger than 1. It assumes that the calling code has done it.
func (d *dictionary) put(b byte) {
	d.buffer[d.w] = b
	d.readLen++
	d.w++
	if d.w >= len(d.buffer) {
		d.full = true
		d.w = 0
	}
}

// putLiteral puts a single literal byte into the dictionary. It checks that
// there is actual space for doing it.
func (d *dictionary) putLiteral(lit byte) error {
	if d.writeLen() < 1 {
		return errDictionaryOverflow
	}
	d.put(lit)
	return nil
}

// get returns the byte at the given distance. Note that the last byte written
// has distance 1.
func (d *dictionary) get(distance int) byte {
	i := d.w - distance
	if i < 0 {
		i += len(d.buffer)
	}
	return d.buffer[i]
}

// copyMatch copies a match. The distance must be in the interval
// [0,maxDistance]. The length must be nonnegative.
func (d *dictionary) copyMatch(distance, length int) error {
	switch {
	case !(0 <= distance && distance <= d.maxDistance()):
		return errDistOutOfRange
	case length < 0:
		return errLengthOutOfRange
	case length > d.writeLen():
		return errDictionaryOverflow
	}
	// can be optimized using copy
	for ; length > 0; length-- {
		d.put(d.get(distance))
	}
	return nil
}

// errDictionaryUnderflow indicates that no bytes can be read from the
// dictionary. Note that the function simply copies data out of the buffer, it
// doesn't add any data to the dictionary.
var errDictionaryUnderflow = errors.New("dictionary underflow")

// Read can be used to access data in the dictionary.
func (d *dictionary) Read(p []byte) (n int, err error) {
	if d.readLen <= 0 {
		return 0, errDictionaryUnderflow
	}
	n = len(p)
	if n > d.readLen {
		n = d.readLen
	}
	for i := 0; i < n; i++ {
		p[i] = d.get(n - i)
	}
	d.readLen -= n
	return n, nil
}
