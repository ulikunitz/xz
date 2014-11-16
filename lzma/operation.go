package lzma

import "fmt"

// operation represents an operation on the dictionary during encoding or
// decoding.
type operation interface {
	applyDecoderDict(d *decoderDict) error
	Len() int
}

// rep represents a repetition at the given distance and the given length
type rep struct {
	length   int
	distance int
}

// applyDecoderDict applies the repetition on the decoder dictionary.
func (r rep) applyDecoderDict(d *decoderDict) error {
	return d.copyMatch(r.distance, r.length)
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

// applyDecoderDict appends the literal to the decoder dictionary.
func (l lit) applyDecoderDict(d *decoderDict) error {
	return d.addByte(l.b)
}

// Len returns 1 for the single byte literal.
func (l lit) Len() int {
	return 1
}

// String returns a string representation for the literal.
func (l lit) String() string {
	return fmt.Sprintf("lit(%02x)", l.b)
}
