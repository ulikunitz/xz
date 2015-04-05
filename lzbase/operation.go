package lzbase

import (
	"fmt"
	"unicode"
)

// operation represents an operation on the dictionary during encoding or
// decoding.
type operation interface {
	applyReaderDict(d *readerDict) error
	Len() int
}

// rep represents a repetition at the given distance and the given length
type match struct {
	// supports all possible distance values, including the eos marker
	distance int64
	length   int
}

// applyReaderDict applies the repetition on the decoder dictionary.
func (m match) applyReaderDict(d *readerDict) error {
	_, err := d.WriteRep(m.distance, m.length)
	return err
}

// Len return the length of the repetition.
func (m match) Len() int {
	return m.length
}

// String returns a string representation for the repetition.
func (m match) String() string {
	return fmt.Sprintf("match{%d,%d}", m.distance, m.length)
}

// lit represents a single byte literal.
type lit struct {
	b byte
}

// applyReaderDict appends the literal to the decoder dictionary.
func (l lit) applyReaderDict(d *readerDict) error {
	return d.WriteByte(l.b)
}

// Len returns 1 for the single byte literal.
func (l lit) Len() int {
	return 1
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
