package lzma

// encoderDict represents the dictionary while encoding LZMA files. It supports
// buffering the data and finding backward matches at the same time.
type encoderDict struct {
	data []byte
	// history length
	h int
	// buffer length
	b int
	// current index
	c int
	// writer index
	w int
	// provides total length
	total int64
	// marks eof for dictionary
	eof bool
}

// newEncoderDict creates a new instance of the decoder dictionary. If the
// arguments are not positive an error is returned.
func newEncoderDict(bufferLen int, historyLen int) (p *encoderDict, err error) {
	if !(0 < bufferLen) {
		return nil, newError("bufferLen must be positive")
	}
	if !(0 < historyLen) {
		return nil, newError("historyLen must be positive")
	}
	h := historyLen
	if h < maxLength {
		h = maxLength
	}
	z := h + bufferLen
	if z < bufferLen {
		return nil, newError(
			"LZMA dictionary size overflows integer range")
	}
	p = &encoderDict{
		data: make([]byte, 0, z),
		h:    historyLen,
		b:    bufferLen,
	}
	return p, nil
}

// Len returns the actual length of data stored in the dictionary.
func (p *encoderDict) Len() int {
	w := p.w
	if w == len(p.data) {
		return p.c
	}
	delta := p.c - w
	if delta <= 0 {
		delta += len(p.data)
	}
	if delta > p.h {
		delta = p.h
	}
	return delta
}

// buffered returns the number of buffered bytes that have not been consumed so
// far
func (p *encoderDict) buffered() int {
	delta := p.w - p.c
	if delta < 0 {
		delta += len(p.data)
	}
	return delta
}

// writable computes the maximum number of bytes supported to be written
func (p *encoderDict) writable() int {
	if p.eof {
		return 0
	}
	n := p.b - p.buffered()
	if n < 0 {
		panic("more data buffered than buffer length")
	}
	return n
}

var errOverflow = newError("overflow")

// Write writes data into the buffer. If the size of p exceeds the number of
// bytes that is writable errOverflow will be returned.
func (p *encoderDict) Write(d []byte) (n int, err error) {
	n = p.writable()
	if n < len(d) {
		err = errOverflow
		d = d[:n]
	} else {
		n = len(d)
	}
	if p.w >= p.c {
		m := cap(p.data) - p.w
		if m > len(d) {
			m = len(d)
		}
		w := p.w + m
		if w > len(p.data) {
			p.data = p.data[:w]
		}
		copy(p.data[p.w:], d[:m])
		if w >= cap(p.data) {
			p.w = 0
		} else {
			p.w = w
		}
		d = d[m:]
	}
	if len(d) > 0 {
		p.w += copy(p.data[p.w:], d)
		if p.w > len(p.data) {
			panic("p.w exceeds len(p.data)")
		}
		if p.w >= cap(p.data) {
			p.w = 0
		}
	}
	return n, errOverflow
}

// move advances the c pointer by n bytes. The argument n must be nonnegative
// and is not allowed to become greater than the bytes actually buffered in the
// encoder dictionary.
func (d *encoderDict) move(n int) (err error) {
	if n < 0 {
		return newError("n must be nonnegative")
	}
	if n > d.buffered() {
		return newError("can't skip more bytes then buffered")
	}
	d.c += n
	if d.c >= len(d.data) {
		d.c -= len(d.data)
	}
	d.total += int64(n)
	return nil
}

// Total returns the total number of bytes written into the dictionary.
func (d *encoderDict) Total() int64 {
	return d.total
}

// GetByte returns the byte at the given distance. If the distance is too big
// zero is returned.
func (d *encoderDict) GetByte(distance int) byte {
	if distance <= 0 {
		panic("distance must be positive")
	}
	if distance > d.Len() {
		return 0
	}
	i := d.c - distance
	if i < 0 {
		i += cap(d.data)
	}
	return d.data[i]
}
