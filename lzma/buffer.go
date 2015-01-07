package lzma

import "io"

var (
	errOffset       = newError("offset outside buffer range")
	errAgain        = newError("buffer exceeded; repeat")
	errNegLen       = newError("length is negative")
	errNrOverflow   = newError("number overflow")
	errCapacity     = newError("capacity must be larger than zero")
	errClosedBuffer = newError("buffer is closed for writing")
)

type buffer struct {
	data       []byte
	start      int64
	cursor     int64
	end        int64
	writeLimit int
	closed     bool
}

func (b *buffer) Cap() int {
	return len(b.data)
}

func (b *buffer) Len() int {
	return int(b.end - b.start)
}

func (b *buffer) Readable() int {
	return int(b.end - b.cursor)
}

func (b *buffer) Writable() int {
	return int(b.cursor + int64(b.writeLimit) - b.end)
}

func (b *buffer) setEnd(x int64) {
	if x < 0 {
		panic("b.end overflow?")
	}
	b.end = x
	b.start = x - int64(len(b.data))
	if b.start < 0 {
		b.start = 0
	}
}

func (b *buffer) index(off int64) int {
	if off < 0 {
		panic("negative offsets are not supported")
	}
	return int(off % int64(len(b.data)))
}

func initBuffer(b *buffer, capacity int) error {
	if capacity <= 0 {
		return errCapacity
	}
	*b = buffer{data: make([]byte, capacity), writeLimit: capacity}
	return nil
}

func newBuffer(capacity int) (b *buffer, err error) {
	b = new(buffer)
	err = initBuffer(b, capacity)
	return
}

func (b *buffer) verifyOffset(off int64) error {
	if !(b.start <= off && off <= b.end) {
		return errOffset
	}
	return nil
}

func (b *buffer) readOff(p []byte, off int64) {
	for len(p) > 0 {
		s := b.index(off)
		m := copy(p, b.data[s:])
		off += int64(m)
		p = p[m:]
	}
}

func (b *buffer) ReadAt(p []byte, off int64) (n int, err error) {
	if off < b.start {
		return 0, errOffset
	}
	k := b.end - off
	n = len(p)
	if k < int64(n) {
		if k < 0 {
			return 0, errOffset
		}
		if b.closed {
			err = io.EOF
		} else {
			err = errAgain
		}
		n = int(k)
	}
	b.readOff(p[:n], off)
	return
}

func (b *buffer) Read(p []byte) (n int, err error) {
	k := b.end - b.cursor
	n = len(p)
	if k < int64(n) {
		if k < 0 {
			panic("wrong b.cursor")
		}
		if k == 0 && b.closed {
			return 0, io.EOF
		}
		n = int(k)
	}
	b.readOff(p[:n], b.cursor)
	b.cursor += int64(n)
	return
}

func (b *buffer) Discard(n int) (discarded int, err error) {
	if n < 0 {
		return 0, errNegLen
	}
	k := b.end - b.cursor
	if k < int64(n) {
		if k < 0 {
			panic("wrong b.cursor")
		}
		if b.closed {
			err = io.EOF
		} else {
			err = errAgain
		}
		n = int(k)
	}
	discarded = n
	b.cursor += int64(n)
	return
}

func (b *buffer) copyNOff(w io.Writer, n int, off int64) (copied int, err error) {
	start, end := off, off+int64(n)
	e := b.index(end)
	for off < end {
		s := b.index(off)
		var q []byte
		if s < e {
			q = b.data[s:e]
		} else {
			q = b.data[s:]
		}
		var m int
		m, err = w.Write(q)
		off += int64(m)
		if err != nil {
			break
		}
	}
	return int(off - start), err
}

func (b *buffer) CopyAt(w io.Writer, n int, off int64) (copied int, err error) {
	if n < 0 {
		return 0, errNegLen
	}
	if off < b.start {
		return 0, errOffset
	}
	k := b.end - off
	if k < int64(n) {
		if k < 0 {
			return 0, errOffset
		}
		if b.closed {
			err = io.EOF
		} else {
			err = errAgain
		}
		n = int(k)
	}
	var cerr error
	copied, cerr = b.copyNOff(w, n, off)
	if cerr != nil {
		err = cerr
	}
	return
}

func (b *buffer) Copy(w io.Writer, n int) (copied int, err error) {
	if n < 0 {
		return 0, errNegLen
	}
	k := b.end - b.cursor
	if k < int64(n) {
		if k < 0 {
			panic("wrong b.cursor")
		}
		if k == 0 && b.closed {
			return 0, io.EOF
		}
		n = int(k)
	}
	copied, err = b.copyNOff(w, n, b.cursor)
	b.cursor += int64(copied)
	return
}

func (b *buffer) ReadByteAt(off int64) (c byte, err error) {
	if !(b.start <= off && off <= b.end) {
		return 0, errOffset
	}
	if off == b.end {
		if b.closed {
			return 0, io.EOF
		}
		return 0, errAgain
	}
	i := b.index(off)
	return b.data[i], nil
}

func (b *buffer) ReadByte() (c byte, err error) {
	if b.cursor == b.end {
		if b.closed {
			return 0, io.EOF
		}
		return 0, errAgain
	}
	i := b.index(b.cursor)
	b.cursor++
	return b.data[i], nil
}

func (b *buffer) writeSlice(p []byte) {
	off := b.end
	for len(p) > 0 {
		i := b.index(off)
		m := copy(b.data[i:], p)
		off += int64(m)
		if off < 0 {
			panic("overflow b.end")
		}
		p = p[m:]
	}
	b.setEnd(off)
}

func (b *buffer) Write(p []byte) (n int, err error) {
	if b.closed {
		return 0, errClosedBuffer
	}
	n = b.Writable()
	if n < len(p) {
		err = errAgain
		p = p[:n]
	}
	b.writeSlice(p)
	return
}

func (b *buffer) WriteByte(c byte) error {
	if b.closed {
		return errClosedBuffer
	}
	if b.Writable() < 1 {
		return errAgain
	}
	i := b.index(b.end)
	b.data[i] = c
	b.setEnd(b.end + 1)
	return nil
}

func (b *buffer) WriteRepOff(n int, off int64) (written int, err error) {
	if n < 0 {
		return 0, errNegLen
	}
	if b.closed {
		return 0, errClosedBuffer
	}
	if !(b.start <= off && off < b.end) {
		return 0, errOffset
	}
	if b.Writable() < n {
		return 0, errAgain
	}
	end := off + int64(n)
	e := b.index(end)
	for off < end {
		s := b.index(off)
		var t int
		if end > b.end {
			t = b.index(b.end)
		} else {
			t = e
		}
		var q []byte
		if s < t {
			q = b.data[s:t]
		} else {
			q = b.data[s:]
		}
		b.writeSlice(q)
		off += int64(len(q))
	}
	return n, nil
}

func (b *buffer) Close() error {
	b.closed = true
	return nil
}

func (b *buffer) EqualBytes(off1, off2 int64, max int) int {
	if off1 < b.start || off2 < b.start {
		return 0
	}
	n := b.end - off1
	if n < int64(max) {
		if n < 1 {
			return 0
		}
		max = int(n)
	}
	n = b.end - off2
	if n < int64(max) {
		if n < 1 {
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
