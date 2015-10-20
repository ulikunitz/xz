// Copyright 2015 Ulrich Kunitz. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package lzma

import (
	"errors"
	"fmt"

	"github.com/ulikunitz/xz/hash"
)

/* For compression we need to find byte sequences that match the byte
 * sequence at the dictionary head. A hash table is a simple method to
 * provide this capability.
 */

// The minimum is somehow arbitrary but the maximum is limited by the
// memory requirements of the hash table.
const (
	minTableExponent = 9
	maxTableExponent = 20
)

// newRoller contains the function used to create an instance of the
// hash.Roller.
var newRoller = func(n int) hash.Roller { return hash.NewCyclicPoly(n) }

// hashTable stores the hash table including the rolling hash method.
//
// We implement chained hashing into a circular buffer. Each entry in
// the circular buffer stores the delta distance to the next position with a
// word that has the same hash value.
type hashTable struct {
	// actual hash table
	t []int64
	// circular list data with the offset to the next word
	data  []uint32
	front int
	rear  int
	// mask for computing the index for the hash table
	mask uint64
	// start offset
	start int64
	// hash offset; initial value is start - int64(wordLen)
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
	e := 30 - nlz32(n)
	switch {
	case e < minTableExponent:
		e = minTableExponent
	case e > maxTableExponent:
		e = maxTableExponent
	}
	return e
}

// newHashTable creates a new hash table for word of length wordLen
func newHashTable(bufCap int, wordLen int) (t *hashTable, err error) {
	if !(0 < bufCap && 0 < bufCap+1) {
		return nil, errors.New(
			"newHashTable: buffer capacity out of range")
	}
	exp := hashTableExponent(uint32(bufCap))
	if !(1 <= wordLen && wordLen <= 4) {
		return nil, errors.New("newHashTable: " +
			"argument wordLen out of range")
	}
	n := 1 << uint(exp)
	if n <= 0 {
		panic("newHashTable: exponent is too large")
	}
	t = &hashTable{
		t:       make([]int64, n),
		data:    make([]uint32, bufCap+1),
		mask:    (uint64(1) << uint(exp)) - 1,
		hoff:    -int64(wordLen),
		wordLen: wordLen,
		wr:      newRoller(wordLen),
		hr:      newRoller(wordLen),
	}
	return t, nil
}

// Reset puts hashTable back into a pristine condition.
func (t *hashTable) Reset(pos int64) error {
	if pos < 0 {
		return errors.New("hashTable.Reset: pos must be non-negative")
	}
	for i := range t.t {
		t.t[i] = 0
	}
	t.front = 0
	t.rear = 0
	t.start = pos
	t.hoff = pos - int64(t.wordLen)
	t.wr = newRoller(t.wordLen)
	t.hr = newRoller(t.wordLen)
	return nil
}

// Pos returns the number of all byte written already to the matcher. We
// call it in the code also absolute position.
func (t *hashTable) Pos() int64 {
	return t.hoff + int64(t.wordLen)
}

// available returns the number of of bytes available for the next
// write.
func (t *hashTable) available() int {
	n := t.rear - 1 - t.front
	if n < 0 {
		n += len(t.data)
	}
	return n
}

// buffered returns the number of bytes that are currently hashed.
func (t *hashTable) buffered() int {
	n := t.front - t.rear
	if n < 0 {
		n += len(t.data)
	}
	return n
}

// addIndex adds n to an index ensuring that is stays inside the
// circular buffer for the hash chain.
func (t *hashTable) addIndex(i, n int) int {
	i += n - len(t.data)
	if i < 0 {
		i += len(t.data)
	}
	return i
}

// putDelta puts the delta instance at the current front of the circular
// chain buffer.
func (t *hashTable) putDelta(delta uint32) error {
	if t.available() < 1 {
		return errNoSpace
	}
	t.data[t.front] = delta
	t.front = t.addIndex(t.front, 1)
	return nil
}

// putEntry puts a new entry into the hash table. If there is already a
// value stored it is moved into the circular chain buffer.
func (t *hashTable) putEntry(h uint64, pos int64) error {
	i := h & t.mask
	old := t.t[h&t.mask] - 1
	t.t[i] = pos + 1
	var delta int64
	if old >= 0 {
		delta = pos - old
		if delta > 1<<32-1 || delta > int64(t.buffered()) {
			delta = 0
		}
	}
	return t.putDelta(uint32(delta))
}

// WriteByte converts a single byte into a hash and puts them into the hash
// table.
func (t *hashTable) WriteByte(b byte) error {
	h := t.wr.RollByte(b)
	t.hoff++
	if t.hoff < t.start {
		return nil
	}
	return t.putEntry(h, t.hoff)
}

// Write converts the bytes provided into hash tables and stores the
// abbreviated offsets into the hash table. The function will never return an
// error.
func (t *hashTable) Write(p []byte) (n int, err error) {
	for i, b := range p {
		if err := t.WriteByte(b); err != nil {
			return i, err
		}
	}
	return len(p), nil
}

// Discard discards data from the circular chain buffer.
func (t *hashTable) Discard(n int) (discarded int, err error) {
	if n < 0 {
		panic("negative argument")
	}
	m := t.buffered()
	if m < n {
		n = m
		err = errors.New("discarded less bytes then requested")
	}
	t.rear = t.addIndex(t.rear, n)
	return n, err
}

// maxMatches limits the number of matches provided by the Matches
// function. This controls the speed of the overall encoding. This limit
// might be better implemented at a higher level, because it doesn't
// care for the current dictionary head.
const maxMatches = 32

// getMatches returns the potential positions for a specific hash.
func (t *hashTable) getMatches(h uint64) (positions []int64) {
	tailPos := t.hoff + 1 - int64(t.buffered())
	if tailPos < 0 {
		panic("tail position negative")
	}
	positions = make([]int64, 0, maxMatches)
	// get the slot for the hash
	pos := t.t[h&t.mask] - 1
	for {
		delta := pos - tailPos
		if delta < 0 {
			return positions
		}
		if pos > t.hoff {
			panic("pos larger than hoff")
		}
		positions = append(positions, pos)
		if len(positions) >= maxMatches {
			return positions
		}
		i := t.rear - len(t.data) + int(delta)
		if i < 0 {
			i += len(t.data)
		}
		u := t.data[i]
		if u == 0 {
			return positions
		}
		pos -= int64(t.data[i])
	}
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
func (t *hashTable) Matches(p []byte) (positions []int64) {
	if len(p) != t.wordLen {
		panic(fmt.Errorf(
			"Matches: byte slice must have length %d", t.wordLen))
	}
	h := t.hash(p)
	return t.getMatches(h)
}
