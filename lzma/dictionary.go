package lzma

import (
	"io"
)

// dictionary provides the dictionary. It maintains a total count of bytes put
// into the dictionary because it is used by the LZMA operation encoding.
type dictionary struct {
	data []byte
	// total count of byte put in the dictionary
	total int64
	// length of history in the dictionary
	historyLen int
}

// init initializes the dictionary. The correctness of the parameters is
// checked.
func (d *dictionary) init(historyLen, capacity int) error {
	if historyLen < 1 {
		return newError("history length must be at least one byte")
	}
	if int64(historyLen) > MaxDictLen {
		return newError("history length must be less than 2^32")
	}
	if historyLen > capacity {
		return newError("historyLen must not exceed capacity")
	}
	// We allocate the whole buffer.
	d.data = make([]byte, capacity)
	d.total = 0
	d.historyLen = historyLen
	return nil
}

// reset sets the dictionary back
func (d *dictionary) reset() {
	d.total = 0
}

// Len returns the number of bytes currently available in the dictionary.
func (d *dictionary) Len() int {
	if d.total < int64(d.historyLen) {
		return int(d.total)
	}
	return d.historyLen
}

// Cap returns the capacity of the dictionary.
func (d *dictionary) Cap() int {
	return len(d.data)
}

// index returns the index into the data array for the given offset.
func (d *dictionary) index(offset int64) int {
	if offset < 0 {
		panic("negative offsets are not supported")
	}
	return int(offset % int64(len(d.data)))
}

// Moves the total counter k points forward. The function assumes that the
// content starting at the old total position is already correct for the next k
// bytes.
func (d *dictionary) advance(k int) {
	if k < 0 {
		panic("k out of range")
	}
	d.total += int64(k)
	if d.total < 0 {
		panic("overflow for d.total")
	}
}

// GetByte returns the byte at the given distance. It returns the zero byte if
// the distance is too large. The latter supports the encoding and decoding of
// match operations directly.
func (d *dictionary) GetByte(distance int) byte {
	if distance <= 0 {
		panic("distance must be positive")
	}
	if distance > d.Len() {
		return 0
	}
	i := d.index(d.total - int64(distance))
	return d.data[i]
}

// Write appends the bytes from p into dictionary. It is always guaranteed that
// the last capacity bytes are stored in the dictionary. The total counter is
// advanced accordingly. The function never returns an error.
func (d *dictionary) Write(p []byte) (n int, err error) {
	n = len(p)
	// No optimization for large arrays.
	for len(p) > 0 {
		t := d.index(d.total)
		k := copy(d.data[t:], p)
		d.advance(k)
		p = p[k:]
	}
	return n, nil
}

// AddByte appends a byte to the dictionary. The function is always successful.
func (d *dictionary) addByte(b byte) error {
	d.Write([]byte{b})
	return nil
}

// copyMatch copies a match on the top of the dictionary.
func (d *dictionary) copyMatch(distance int64, length int) error {
	if !(1 <= distance && distance <= int64(d.Len())) {
		return newError("distance out of range [1,d.Len()]")
	}
	if !(1 <= length && length < maxLength) {
		return newError("length out of range [1,maxLength]")
	}
	for length > 0 {
		src := d.total - distance
		end := src + int64(length)
		if end > d.total {
			end = d.total
		}
		s, e := d.index(src), d.index(end)
		if s >= e {
			e = len(d.data)
		}
		k, _ := d.Write(d.data[s:e])
		length -= k
	}
	return nil
}

var errAgain = newError("not enough data in buffer")

// ReadAt reads data from the history. The offset must be inside the actual
// history.
func (d *dictionary) ReadAt(p []byte, off int64) (n int, err error) {
	if off < d.total-int64(d.Len()) {
		return 0, newError("offset outside of history")
	}
	end := off + int64(len(p))
	if end > d.total {
		err = errAgain
		end = d.total
	}
	n = int(end - off)
	p = p[:n]
	e := d.index(end)
	for len(p) > 0 {
		s := d.index(end - int64(len(p)))
		var q []byte
		if s < e {
			q = d.data[s:e]
		} else {
			q = d.data[s:]
		}
		m := copy(p, q)
		p = p[m:]

	}
	return n, nil
}

type readerDict struct {
	dictionary
	bufferLen int
	off       int64
	closed    bool
}

func newReaderDict(historyLen, bufferLen int) (r *readerDict, err error) {
	if historyLen < 1 {
		return nil, newError("history length must be at least one byte")
	}
	if int64(historyLen) > MaxDictLen {
		return nil, newError("history length must be less than 2^32")
	}
	if bufferLen < 1 {
		return nil, newError("bufferLen must at least support 1 byte")
	}
	// There should be enough capacity for a single match.
	capacity := 1 + maxLength
	if historyLen > capacity {
		capacity = historyLen
	}
	if bufferLen > capacity {
		capacity = bufferLen
	}
	r = &readerDict{bufferLen: bufferLen}
	r.dictionary.init(capacity, capacity)
	return r, nil
}

func (r *readerDict) reopen() {
	r.closed = false
	r.off = r.total
}

func (r *readerDict) reset() {
	r.dictionary.reset()
	r.reopen()
}

func (r *readerDict) readable() int {
	return int(r.total - r.off)
}

func (r *readerDict) writable() int {
	if r.closed {
		return 0
	}
	return r.Cap() - r.readable()
}

func (r *readerDict) Read(p []byte) (n int, err error) {
	n, err = r.ReadAt(p, r.off)
	r.off += int64(n)
	switch {
	case n == 0 && r.closed && r.off >= r.total:
		err = io.EOF
	case err == errAgain:
		err = nil
	}
	return
}
