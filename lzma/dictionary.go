package lzma

import "errors"

type dictionary struct {
	buffer  []byte
	readLen int
	w       int
	full    bool
}

func (d *dictionary) reset() {
	d.readLen = 0
	d.w = 0
	d.full = false
}

func (d *dictionary) init(size int) {
	d.buffer = make([]byte, size)
	d.reset()
}

func (d *dictionary) writeLen() int {
	return len(d.buffer) - d.readLen
}

func (d *dictionary) maxDistance() int {
	if d.full {
		return len(d.buffer)
	}
	return d.w
}

var errDictionaryOverflow = errors.New("dictionary will overflow")
var errDistOutOfRange = errors.New("distance out of range")
var errLengthOutOfRange = errors.New("length out of range")

func (d *dictionary) put(b byte) {
	d.buffer[d.w] = b
	d.readLen++
	d.w++
	if d.w >= len(d.buffer) {
		d.full = true
		d.w = 0
	}
}

func (d *dictionary) putLiteral(lit byte) error {
	if d.writeLen() < 1 {
		return errDictionaryOverflow
	}
	d.put(lit)
	return nil
}

func (d *dictionary) get(distance int) byte {
	i := d.w - distance
	if i < 0 {
		i += len(d.buffer)
	}
	return d.buffer[i]
}

func (d *dictionary) copyMatch(distance, length int) error {
	switch {
	case !(0 <= distance && distance <= d.maxDistance()):
		return errDistOutOfRange
	case length < 0:
		return errLengthOutOfRange
	case length > d.writeLen():
		return errDictionaryOverflow
	}
	for ; length > 0; length-- {
		d.put(d.get(distance))
	}
	return nil
}

var errDictionaryUnderflow = errors.New("dictionary underflow")

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
