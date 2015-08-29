package lzma

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

// decoderDict provides the sliding window dictionary as well as the
// circular buffer for reading.
type decoderDict struct {
	buffer
	// absolute position of the dictionary head
	head int64
	// capacity of the dictionary -- the specification calls this
	// dictionary size
	capacity int
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
	*d = decoderDict{capacity: dictCap}
	return initBuffer(&d.buffer, bufCap)
}

// Head returns the position of the dictionary head.
func (d *decoderDict) Head() int64 { return d.head }

// Len returns the actual length of the dictionary.
func (d *decoderDict) Len() int {
	if d.head >= int64(d.capacity) {
		return d.capacity
	}
	return int(d.head)
}

// ByteAt returns a byte stored in the dictionary. If the distance is
// non-positive or exceeds the current length of the dictionary the zero
// byte is returned.
func (d *decoderDict) ByteAt(dist int) byte {
	if !(0 < dist && dist <= d.Len()) {
		return 0
	}
	i := d.front - dist
	if i < 0 {
		i += len(d.data)
	}
	return d.data[i]
}

// WriteMatch writes the match at the top of the dictionary. The given
// distance must point in the current dictionary and the length must not
// exceed the maximum length 273 supported in LZMA.
//
// The error value errNoSpace indicates that no space is available in
// the dictionary for writing. You need to read from the dictionary.
func (d *decoderDict) WriteMatch(dist int, length int) error {
	if !(0 < dist && dist <= d.Len()) {
		return errors.New("WriteMatch: distance out of range")
	}
	if !(0 < length && length <= MaxMatchLen) {
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
		if _, err := d.Write(p); err != nil {
			panic(fmt.Errorf("Write returned error %s", err))
		}
		length -= len(p)
	}
	return nil
}

// WriteByte writes a single byte into the dictionary.
func (d *decoderDict) WriteByte(c byte) error {
	if err := d.buffer.WriteByte(c); err != nil {
		return err
	}
	d.head++
	return nil
}

// Reset resets the dictionary. The read buffer is not changed data, so
// data decoded will remain readable.
func (d *decoderDict) Reset() {
	d.head = 0
}
