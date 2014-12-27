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
		data: make([]byte, 0, z),
		h:    historyLen,
		b:    bufferLen,
	}
	return p, nil
}
