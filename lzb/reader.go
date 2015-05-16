package lzb

import "io"

// Reader provides a basic LZMA reader. It doesn't support any header
// but allows a reset keeping the state.
type Reader struct {
	State    *State
	rd       *rangeDecoder
	buf      *buffer
	readHead int64
	closed   bool
}

func (r *Reader) Restart(raw io.Reader) {
	panic("TODO")
}

func (r *Reader) ResetState() {
	panic("TODO")
}

func (r *Reader) ResetProperties(p Properties) {
	panic("TODO")
}

func (r *Reader) ResetDictionary(p Properties) {
	panic("TODO")
}
