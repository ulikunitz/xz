// Copyright 2015 Ulrich Kunitz. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package newlzma

import (
	"errors"
	"fmt"

	"github.com/uli-go/xz/basics/u32"
	"github.com/uli-go/xz/hash"
)

/* For compression we need to find byte sequences that match the byte
 * sequence at the dictionary head. A hash table is a simple method to
 * provide this capability.
 */

// slotEntries gives the number of entries in one slot of the hash table. If
// slotEntries is larger than 128 the representation of fields a and b in
// slot must be reworked.
const slotEntries = 24

// The minTableExponent give the minimum and maximum for the table exponent.
// The minimum is somehow arbitrary but the maximum is limited by the
// memory requirements of the hash table.
const (
	minTableExponent = 9
	maxTableExponent = 20
)

// newRoller contains the function used to create an instance of the
// hash.Roller.
var newRoller = func(n int) hash.Roller { return hash.NewCyclicPoly(n) }

// slot defines the data structure for a slot in the hash table. The number of
// entries is given by slotEntries constant.
type slot struct {
	entries [slotEntries]uint32
	// start index; bit 7 set if non-empty
	a uint8
	// next entry to overwrite
	b uint8
}

const slotFilled uint8 = 0x80

// start returns the start index of the slot
func (s *slot) start() int {
	return int(s.a &^ slotFilled)
}

// end returns the end index of the slot
func (s *slot) end() int {
	return int(s.b)
}

// empty returns true if nothing is stored in the slot
func (s *slot) empty() bool {
	return s.a&slotFilled == 0
}

// PutEntry puts an entry into a slot.
func (s *slot) PutEntry(u uint32) {
	a, b := s.start(), s.end()
	s.entries[b] = u
	bp1 := (b + 1) % slotEntries
	if a == b && !s.empty() {
		a, b = bp1, bp1
	} else {
		b = bp1
	}
	s.a = slotFilled | uint8(a)
	s.b = uint8(b)
}

// Reset puts the slot back into a pristine condition.
func (s *slot) Reset() {
	s.a, s.b = 0, 0
}

// hashTable stores the hash table including the rolling hash method.
type hashTable struct {
	// actual hash table
	t []slot
	// mask for computing the index for the hash table
	mask uint64
	// hash offset
	hoff int64
	// length of the hashed word
	wordLen int
	// hash roller for computing the hash values for the Write
	// method
	wr hash.Roller
	// hash roller for computing arbitrary hashes
	hr hash.Roller
}

// hashTableExponent derives the hash table exponent from the dictionary
// capacity.
func hashTableExponent(n uint32) int {
	e := 30 - u32.NLZ(n)
	switch {
	case e < minTableExponent:
		e = minTableExponent
	case e > maxTableExponent:
		e = maxTableExponent
	}
	return e
}

// newHashTable creates a new hash table for word of length wordLen
func newHashTable(dictCap int, wordLen int) (t *hashTable, err error) {
	if dictCap < 1 {
		return nil, errors.New(
			"newHashTable: dictCap must be larger than 1")
	}
	if dictCap > maxDictCap {
		return nil, errors.New(
			"newHashTable: dictCap exceeds supported maximum")
	}
	exp := hashTableExponent(uint32(dictCap))
	if !(1 <= wordLen && wordLen <= 4) {
		return nil, errors.New("newHashTable: " +
			"argument wordLen out of range")
	}
	slotLen := 1 << uint(exp)
	if slotLen <= 0 {
		panic("newHashTable: exponent is too large")
	}
	t = &hashTable{
		t:       make([]slot, slotLen),
		mask:    (uint64(1) << uint(exp)) - 1,
		hoff:    -int64(wordLen),
		wordLen: wordLen,
		wr:      newRoller(wordLen),
		hr:      newRoller(wordLen),
	}
	return t, nil
}

// Reset puts hashTable back into a pristine condition.
func (t *hashTable) Reset() {
	for i := range t.t {
		t.t[i].Reset()
	}
	t.hoff = -int64(t.wordLen)
	t.wr = newRoller(t.wordLen)
	t.hr = newRoller(t.wordLen)
}

func (t *hashTable) Pos() int64 {
	return t.hoff + int64(t.wordLen)
}

// putEntry puts an entry into the hash table using the given hash.
func (t *hashTable) putEntry(h uint64, u uint32) {
	t.t[h&t.mask].PutEntry(u)
}

// WriteByte converts a single byte into a hash and puts them into the hash
// table.
func (t *hashTable) WriteByte(b byte) error {
	h := t.wr.RollByte(b)
	t.hoff++
	if t.hoff >= 0 {
		t.putEntry(h, uint32(t.hoff))
	}
	return nil
}

// Write converts the bytes provided into hash tables and stores the
// abbreviated offsets into the hash table. The function will never return an
// error.
func (t *hashTable) Write(p []byte) (n int, err error) {
	for _, b := range p {
		t.WriteByte(b)
	}
	return len(p), nil
}

// posU32 converts the unsigned uint32 value to an int64 position.
func (t *hashTable) posU32(u uint32) int64 {
	h := t.hoff &^ (1<<32 - 1)
	p := h + int64(u)
	if p > t.hoff {
		p -= (1 << 32)
	}
	return p
}

// getMatches returns the potential positions for a specific hash.
func (t *hashTable) getMatches(h uint64) (positions []int64) {
	// get the slot for the hash
	s := &t.t[h&t.mask]
	if s.empty() {
		return nil
	}
	positions = make([]int64, 0, slotEntries)
	appendPositions := func(p []uint32) {
		for _, u := range p {
			pos := t.posU32(u)
			positions = append(positions, pos)
		}
	}
	a, b := s.start(), s.end()
	if a >= b {
		appendPositions(s.entries[a:])
		appendPositions(s.entries[:b])
	} else {
		appendPositions(s.entries[a:b])
	}
	return positions
}

// hash computes the rolling hash for the word stored in p. For correct
// results its length must be equal to t.wordLen.
func (t *hashTable) hash(p []byte) uint64 {
	var h uint64
	for _, b := range p {
		h = t.hr.RollByte(b)
	}
	return h
}

// WordLen returns the length of the words supported by the Matches
// function.
func (t *hashTable) WordLen() int {
	return t.wordLen
}

// Matches returns the positions of potential matches. Those matches
// have to be verified, whether they are indeed matching. The byte slice
// p must have word length of the hash table.
func (t *hashTable) Matches(p []byte) (positions []int64, err error) {
	if len(p) != t.wordLen {
		return nil, fmt.Errorf(
			"Matches: byte slice must have length %d", t.wordLen)
	}
	h := t.hash(p)
	return t.getMatches(h), nil
}
