package lzb

import "errors"

type buffer struct {
	data       []byte
	bottom     int64 // bottom == max(top - len(data), 0)
	top        int64
	writeLimit int64
}

const maxWriteLimit = 1<<63 - 1

var (
	errOffset = errors.New("offset outside buffer range")
	errAgain  = errors.New("buffer overflow; repeat")
	errNegLen = errors.New("length is negative")
	errLimit  = errors.New("write limit reached")
)

func initBuffer(b *buffer, capacity int) {
	*b = buffer{data: make([]byte, capacity), writeLimit: maxWriteLimit}
}

func newBuffer(capacity int) *buffer {
	b := new(buffer)
	initBuffer(b, capacity)
	return b
}

func (b *buffer) capacity() int {
	return len(b.data)
}

func (b *buffer) length() int {
	return int(b.top - b.bottom)
}

func (b *buffer) setTop(off int64) {
	if off < 0 {
		panic("b.Top overflow?")
	}
	if off > b.writeLimit {
		panic("off exceeds writeLimit")
	}
	b.top = off
	b.bottom = off - int64(len(b.data))
	if b.bottom < 0 {
		b.bottom = 0
	}
}

func (b *buffer) index(off int64) int {
	if off < 0 {
		panic("negative offset?")
	}
	return int(off % int64(len(b.data)))
}

func (b *buffer) Write(p []byte) (n int, err error) {
	if len(p) == 0 {
		return
	}
	var m int
	off := b.top
	if off+int64(len(p)) > b.writeLimit {
		m = int(b.writeLimit - off)
		p = p[:m]
		err = errors.New("write limit reached")
	}
	m = len(p) - len(b.data)
	if m > 0 {
		n += m
		p = p[m:]
	}
	for len(p) > 0 {
		m = copy(b.data[b.index(off):], p)
		n += m
		p = p[m:]
	}
	b.setTop(off + int64(n))
	return n, err
}

func (b *buffer) WriteByte(c byte) error {
	if b.top >= b.writeLimit {
		return errLimit
	}
	b.data[b.index(b.top)] = c
	b.setTop(b.top + 1)
	return nil
}

func (b *buffer) writeRep(off int64, n int) (written int, err error) {
	if n < 0 {
		return 0, errNegLen
	}
	if !(b.bottom <= off && off <= b.top) {
		return 0, errOffset
	}
	start, end := off, off + int64(n)
	if !(end <= b.top) {
		return 0, errAgain
	}
	e := b.index(end)
	for off < end {
		s := b.index(off)
		var q []byte
		if s < e {
			q = b.data[s:e]
		} else {
			q = b.data[s:]
		}
		n, err := b.Write(q)
		off += int64(n)
		if err != nil {
			break
		}
		off += int64(n)
	}
	b.setTop(off)
	return int(off - start), nil
}

// equalBytes count the equal bytes at off1 and off2 until max is reached.
func (b *buffer) equalBytes(off1, off2 int64, max int) int {
	if off1 < b.top || off2 < b.top || max <= 0 {
		return 0
	}
	n := b.top - off1
	if n < int64(max) {
		if n <= 0 {
			return 0
		}
		max = int(n)
	}
	n = b.top - off2
	if n < int64(max) {
		if n <= 0 {
			return 0
		}
		max = int(n)
	}
	for k := 0; k < max; k++ {
		i, j := b.index(off1+int64(k)), b.index(off2+int64(k))
		if b.data[i] != b.data[j] {
			return k
		}
	}
	return max
}
