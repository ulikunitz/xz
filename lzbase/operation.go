package lzbase

import (
	"fmt"
	"unicode"
)

// operation represents an operation on the dictionary during encoding or
// decoding.
type Operation interface {
	Len() int
	Apply(dict *ReaderDict) error
}

// rep represents a repetition at the given distance and the given length
type match struct {
	// supports all possible distance values, including the eos marker
	distance int64
	// length
	n int
}

// eosMatch may mark the end of an LZMA stream.
var eosMatch = match{distance: maxDistance, n: MinLength}

// Len returns the number of bytes matched.
func (m match) Len() int {
	return m.n
}

// Apply writes the repetition match into the dictionary.
func (m match) Apply(dict *ReaderDict) error {
	_, err := dict.writeRep(m.distance, m.n)
	return err
}

// String returns a string representation for the repetition.
func (m match) String() string {
	return fmt.Sprintf("match{%d,%d}", m.distance, m.n)
}

// lit represents a single byte literal.
type lit struct {
	b byte
}

// Len returns 1 for the single byte literal.
func (l lit) Len() int {
	return 1
}

// Apply writes the literal byte into the dictionary.
func (l lit) Apply(dict *ReaderDict) error {
	return dict.writeByte(l.b)
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
