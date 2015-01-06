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

// advbance moves the total counter k points forward. The function assumes that
// the content starting at the old total position is already correct for the
// next k bytes.
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

// WriteAt stores data at a specific position in the dictionary. The function
// doesn't return an error even if p is bigger than the capacity.
func (d *dictionary) WriteAt(p []byte, off int64) (n int, err error) {
	n = len(p)
	// No optimization for large arrays.
	for len(p) > 0 {
		i := d.index(off)
		k := copy(d.data[i:], p)
		off += int64(k)
		if off < 0 {
			panic("overflow off")
		}
		p = p[k:]
	}
	return
}

// Write appends the bytes from p into dictionary. The total counter is
// advanced accordingly. The function never returns an error even if not the
// whole array can be stored in the dictionary.
func (d *dictionary) Write(p []byte) (n int, err error) {
	n, err = d.WriteAt(p, d.total)
	d.total += int64(n)
	return
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
	src := d.total - int64(distance)
	end := src + int64(length)
	e := d.index(end)
	for length > 0 {
		s := d.index(end - int64(length))
		var t int
		if end > d.total {
			t = d.index(d.total)
		} else {
			t = e
		}
		if s >= t {
			t = len(d.data)
		}
		k, _ := d.Write(d.data[s:t])
		length -= k
	}
	return nil
}

// errAgain indicates that there is not enough data and the call should be
// repeated.
var errAgain = newError("not enough data; repeat")

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

// readerDict represents a reader dictionary for reading. It maintains another
// reader counter.
type readerDict struct {
	dictionary
	bufferLen int
	off       int64
	closed    bool
}

// newReaderDict created a new reader dict value.
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
	capacity := historyLen
	if bufferLen > capacity {
		capacity = bufferLen
	}
	r = &readerDict{bufferLen: bufferLen}
	if err = r.dictionary.init(capacity, capacity); err != nil {
		return nil, err
	}
	return r, nil
}

// reopen reopens a reader dictionary
func (r *readerDict) reopen() {
	r.closed = false
	r.off = r.total
}

// reset resets the reader dictionary. If it has been closed it will be opened
// again.
func (r *readerDict) reset() {
	r.dictionary.reset()
	r.reopen()
}

// readable returns the number of readable bytes.
func (r *readerDict) readable() int {
	return int(r.total - r.off)
}

// writable returns the number of writable bytes. For a closed reader
// dictionary not bytes will be writable.
func (r *readerDict) writable() int {
	if r.closed {
		return 0
	}
	return r.Cap() - r.readable()
}

// Read reads the given byte slice from the reader dictionary. The reader
// offset will be updated.
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

// writerDict is used for encoding LZMA files.
type writerDict struct {
	dictionary
	bufferLen int
	off       int64
}

// newWriterDict creates a new writer dictionary. The capacity of the buffer
// will be the total of historyLen and bufferLen.
func newWriterDict(historyLen, bufferLen int) (w *writerDict, err error) {
	if historyLen < 1 {
		return nil, newError("history length must be at least one byte")
	}
	if int64(historyLen) > MaxDictLen {
		return nil, newError("history length must be less than 2^32")
	}
	if bufferLen < 1 {
		return nil, newError("bufferLen must at least support 1 byte")
	}
	capacity := historyLen + bufferLen
	w = &writerDict{bufferLen: bufferLen}
	if err = w.dictionary.init(historyLen, capacity); err != nil {
		return nil, err
	}
	return w, err
}

// buffered returns the number of buffered byted. The buffer is not part of the
// history.
func (w *writerDict) buffered() int {
	return int(w.off - w.total)
}

// Write copies date in slice into the writer dictionary. There must be enough
// space available. If there is not enough space in the buffer errAgain is
// returned.
func (w *writerDict) Write(p []byte) (n int, err error) {
	n = int(w.total + int64(w.bufferLen) - w.off)
	if n < len(p) {
		err = errAgain
		p = p[:n]
	}
	n, _ = w.dictionary.WriteAt(p, w.off)
	w.off += int64(n)
	return
}
