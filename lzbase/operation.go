package lzbase

import (
	"fmt"
	"unicode"
)

// operation represents an operation on the dictionary during encoding or
// decoding.
type operation interface {
	Len() int
	apply(dict *ReaderDict) error
}

// rep represents a repetition at the given distance and the given length
type match struct {
	// supports all possible distance values, including the eos marker
	distance int64
	length   int
}

// eosMatch may mark the end of an LZMA stream.
var eosMatch = match{distance: maxDistance, length: MinLength}

// Len return the length of the repetition.
func (m match) Len() int {
	return m.length
}

// apply writes the repetition match into the dictionary.
func (m match) apply(dict *ReaderDict) error {
	_, err := dict.WriteRep(m.distance, m.length)
	return err
}

// String returns a string representation for the repetition.
func (m match) String() string {
	return fmt.Sprintf("match{%d,%d}", m.distance, m.length)
}

// lit represents a single byte literal.
type lit struct {
	b byte
}

// Len returns 1 for the single byte literal.
func (l lit) Len() int {
	return 1
}

// apply writes the litaral byte into the dictionary.
func (l lit) apply(dict *ReaderDict) error {
	return dict.WriteByte(l.b)
}

// String returns a string representation for the literal.
func (l lit) String() string {
	var c byte
	if unicode.IsPrint(rune(l.b)) {
		c = l.b
	} else {
		c = '.'
	}
	return fmt.Sprintf("lit{%02x %c}", l.b, c)
}
