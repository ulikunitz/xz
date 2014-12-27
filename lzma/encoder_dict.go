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
	// marks eof
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
	if z < bufferLen || z < historyLen {
		return nil, newError(
			"LZMA dictionary size overflows integer range")
	}
	p = &encoderDict{
		data: make([]byte, z),
		h:    historyLen,
		b:    bufferLen,
	}
	return p, nil
}

// buffered returns the number of buffered bytes that have not been consumed so
// far
func (d *encoderDict) buffered() int {
	delta := d.w - d.c
	if delta < 0 {
		delta += len(d.data)
	}
	return delta
}

// writable computes the maximum number of bytes supported to be written
func (d *encoderDict) writable() int {
	if d.eof {
		return 0
	}
	n := d.b - d.buffered()
	if n < 0 {
		panic("more data buffered than buffer length")
	}
	return n
}

// Write writes data into the buffer. If not all bytes can be written the
// number of bytes written and the error code errOverflow is returned.
func (d *encoderDict) Write(p []byte) (n int, err error) {
	n = d.writable()
	if n < len(p) {
		err = errOverflow
	} else {
		n = len(p)
	}
	p = p[:n]
	if d.w >= d.c {
		m := len(d.data) - d.w
		if m > len(p) {
			m = len(p)
		}
		copy(d.data[d.w:], p[:m])
		d.w += m
		if d.w == len(d.data) {
			d.w = 0
		}
		p = p[m:]
	}
	if len(p) > 0 {
		m := copy(d.data[d.w:], p)
		d.w += m
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
	d.c += n - len(d.data)
	if d.c < 0 {
		d.c += len(d.data)
	}
	d.total += int64(n)
	return nil
}
