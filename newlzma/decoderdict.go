package newlzma

import (
	"errors"
	"fmt"
)

// Minimum and maximum values for the dictionary capacity that is called
// dictionary size by the LZMA specification.
const (
	minDictCap = 4096
	maxDictCap = 1<<32 - 1
)

// maximum length for a match
const maxMatchLen = 273

// decoderDict manages the dictionary for the decoder and acts also as a
// reader.
type decoderDict struct {
	// store for the dictionary data
	data []byte
	// absolute position of the dictionary head^
	head int64
	// front pointer into circular buffer
	front int
	// current dictionary length
	n int
	// capacity of the dictionary -- the specification calls this
	// dictionary size
	capacity int
	// length of data buffered for reading
	r int
}

// initDecoderDict initializes  the decoderDict structure.
//
// A typical new function is not provided because the structure will be
// used embedded in the decompressor structure.
func initDecoderDict(d *decoderDict, dictCap, bufCap int) error {
	// lower limit supports easy test cases
	if !(1 <= dictCap && int64(dictCap) <= maxDictCap) {
		return errors.New("initDecoderDict: dictCap out of range")
	}
	if !(dictCap <= bufCap) {
		return errors.New("initDecoderDict: buffer capacity must " +
			"be greater equal than the dictionary " +
			"capacity")
	}
	*d = decoderDict{
		data:     make([]byte, bufCap),
		capacity: dictCap,
	}
	return nil
}

// Head returns the position of the dictionary head.
func (d *decoderDict) Head() int64 { return d.head }

// Len returns the actual length of the dictionary.
func (d *decoderDict) Len() int {
	return d.n
}

// Available returns the number of bytes available for writing.
func (d *decoderDict) Available() int {
	return len(d.data) - d.r
}

// Buffered returns the number of bytes available for reading.
func (d *decoderDict) Buffered() int {
	return d.r
}

// ByteAt returns a byte stored in the dictionary. If the distance is
// non-positive or exceeds the current length of the dictionary the zero
// byte is returned.
func (d *decoderDict) ByteAt(dist int) byte {
	if !(0 < dist && dist <= d.n) {
		return 0
	}
	i := d.front - dist
	if i < 0 {
		i += len(d.data)
	}
	return d.data[i]
}

func (d *decoderDict) put(p []byte) {
	if len(p) >= len(d.data) {
		panic("decodedDict.put: " +
			"p slice too large for dictionary buffer")
	}
	k := copy(d.data[d.front:], p)
	if k < len(p) {
		copy(d.data, p[k:])
	}
	// substraction of len(d.data) prevents overflow
	d.front += len(p) - len(d.data)
	if d.front < 0 {
		d.front += len(d.data)
	}
	d.n += len(p)
	// d.n < 0 is a check for overflow
	if d.n > d.capacity || d.n < 0 {
		d.n = d.capacity
	}
	d.r += len(p)
	if d.r > len(d.data) || d.r < 0 {
		d.r = len(d.data)
	}
}

// errNoSpace indicates that no space is available in the dictionary for
// writing. You need to read from the dictionary.
var errNoSpace = errors.New("not enough space in dictionary")

// WriteMatch writes the match at the top of the dictionary. The given
// distance must point in the current dictionary and the length must not
// exceed the maximum length 273 supported in LZMA.
func (d *decoderDict) WriteMatch(dist int, length int) error {
	if !(0 < dist && dist <= d.n) {
		return errors.New("WriteMatch: distance out of range")
	}
	if !(0 < length && length <= maxMatchLen) {
		return errors.New("WriteMatch: length out of range")
	}
	if length > d.Available() {
		return errNoSpace
	}
	d.head += int64(length)

	i := d.front - dist
	if i < 0 {
		i += len(d.data)
	}
	for length > 0 {
		var p []byte
		if i >= d.front {
			p = d.data[i:]
			i = 0
		} else {
			p = d.data[i:d.front]
			i = d.front
		}
		if len(p) > length {
			p = p[:length]
		}
		d.put(p)
		length -= len(p)
	}
	return nil
}

// WriteByte writes a single byte into the dictionary.
func (d *decoderDict) WriteByte(c byte) error {
	if d.Available() < 1 {
		return errNoSpace
	}
	d.data[d.front] = c
	d.front++
	if d.front == len(d.data) {
		d.front = 0
	}
	if d.n < d.capacity {
		d.n++
	}
	d.r++
	d.head++
	return nil
}

// Read reads data out of the dictionary. Read will not return an error
// if the dictionary head is reached, but the number of bytes read may
// be less then len(p) including zero.
func (d *decoderDict) Read(p []byte) (n int, err error) {
	n = len(p)
	if d.r < n {
		n = d.r
		p = p[:n]
	}
	i := d.front - d.r
	if i < 0 {
		i += len(d.data)
	}
	k := copy(p, d.data[i:])
	if k < n {
		copy(p[k:], d.data)
	}
	d.r -= n
	return n, nil
}

// Reset resets the dictionary. The read buffer is not changed data will
// still be readable.
func (d *decoderDict) Reset() {
	d.head = 0
	d.n = 0
}

func (d *decoderDict) peek() []byte {
	p := make([]byte, d.r)
	k, err := d.Read(p)
	if err != nil {
		panic(fmt.Errorf("peek: "+
			"Read returned unexpected error %s", err))
	}
	if k != len(p) {
		panic(fmt.Errorf("peek: "+
			"Read returned %d; wanted %d", k, len(p)))
	}
	// reset effect of Read
	d.r = len(p)
	return p
}
