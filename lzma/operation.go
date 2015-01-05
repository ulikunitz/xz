package lzma

import "fmt"

// operation represents an operation on the dictionary during encoding or
// decoding.
type operation interface {
	applyReaderDict(d *readerDict) error
	Len() int
}

// rep represents a repetition at the given distance and the given length
type rep struct {
	// supports all possible distance values, including the eos marker
	distance int64
	length   int
}

// applyReaderDict applies the repetition on the decoder dictionary.
func (r rep) applyReaderDict(d *readerDict) error {
	if d.writable() >= r.length {
		return d.copyMatch(r.distance, r.length)
	}
	return newError("insufficient space in reader dictionary")
}

// Len return the length of the repetition.
func (r rep) Len() int {
	return r.length
}

// String returns a string representation for the repetition.
func (r rep) String() string {
	return fmt.Sprintf("rep(%d,%d)", r.distance, r.length)
}

// lit represents a single byte literal.
type lit struct {
	b byte
}

// applyReaderDict appends the literal to the decoder dictionary.
func (l lit) applyReaderDict(d *readerDict) error {
	if d.writable() >= 1 {
		return d.addByte(l.b)
	}
	return newError("insufficient space in reader dictionary")
}

// Len returns 1 for the single byte literal.
func (l lit) Len() int {
	return 1
}

// String returns a string representation for the literal.
func (l lit) String() string {
	return fmt.Sprintf("lit(%02x %c)", l.b, l.b)
}
