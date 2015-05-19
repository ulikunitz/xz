package lzb

import (
	"errors"

	"github.com/uli-go/xz/hash"
)

/* For compression we need to find byte sequences that match the current byte
 * byte sequences in the available dictionary.
 *
 * The simplest way to achieve that are hash tables. While the hash table
 * implementation shouldn't know that, it will support hashes for two-byte and
 * four-byte strings. A hash is a uint64 number and the hash table maps that to
 * a uint32 value.
 */

// slotEntries gives the number of entries in one slot of the hash table. If
// slotEntries is larger than 128 the representation of fields i_ and j_ in
// slot must be reworked.
const slotEntries = 24

// The minTableExponent give the minimum and maximum for the table exponent.
// The minimum is somehow arbitrary but the maximum is defined by the number of
// bits in the hash value.
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

const slotFull uint8 = 0x80

// start returns the start index of the slot
func (s *slot) start() int {
	return int(s.a &^ slotFull)
}

// end returns the end index of the slot
func (s *slot) end() int {
	return int(s.b)
}

// empty returns true if nothing is stored in the slot
func (s *slot) empty() bool {
	return s.a&slotFull == 0
}

// getEntries returns all entries of a slot in the sequence that they have been
// entered.
func (s *slot) getEntries() []uint32 {
	if s.empty() {
		return nil
	}
	r := make([]uint32, 0, slotEntries)
	a, b := s.start(), s.end()
	if a >= b {
		r = append(r, s.entries[a:]...)
		r = append(r, s.entries[:b]...)
	} else {
		r = append(r, s.entries[a:b]...)
	}
	return r
}

// putEntry puts an entry into a slot.
func (s *slot) putEntry(u uint32) {
	a, b := s.start(), s.end()
	s.entries[b] = u
	bp1 := (b + 1) % slotEntries
	if a == b && !s.empty() {
		a, b = bp1, bp1
	} else {
		b = bp1
	}
	s.a = slotFull | uint8(a)
	s.b = uint8(b)
}

// hashTable stores the hash table including the rolling hash method.
type hashTable struct {
	t []slot
	// exponent used to compute the hash table size
	exp  int
	mask uint64
	// historySize
	hlen int64
	// hashOffset
	hoff int64
	wr   hash.Roller
	hr   hash.Roller
}

// hashTableExponent derives the hash table exponent from the history length.
func hashTableExponent(n uint32) int {
	e := 30 - NLZ32(n)
	switch {
	case e < minTableExponent:
		e = minTableExponent
	case e > maxTableExponent:
		e = maxTableExponent
	}
	return e
}

// newHashTable creates a new hash table for n-byte sequences.
func newHashTable(historySize int64, n int) (t *hashTable, err error) {
	if historySize < 1 {
		return nil, errors.New("history length must be at least one byte")
	}
	if historySize > MaxDictSize {
		return nil, errors.New("history length must be less than 2^32")
	}
	exp := hashTableExponent(uint32(historySize))
	if !(1 <= n && n <= 4) {
		return nil, errors.New("argument n out of range")
	}
	slotLen := int(1) << uint(exp)
	if slotLen <= 0 {
		return nil, errors.New("exponent is too large")
	}
	t = &hashTable{
		t:    make([]slot, slotLen),
		exp:  exp,
		mask: (uint64(1) << uint(exp)) - 1,
		hlen: historySize,
		hoff: -int64(n),
		wr:   newRoller(n),
		hr:   newRoller(n),
	}
	return t, nil
}

// SliceLen returns the slice length.
func (t *hashTable) SliceLen() int { return t.wr.Len() }

// putEntry puts an entry into the hash table using the given hash.
func (t *hashTable) putEntry(h uint64, u uint32) {
	t.t[h&t.mask].putEntry(u)
}

// getEntries returns all the values that cant be found in the hash table.
func (t *hashTable) getEntries(h uint64) []uint32 {
	return t.t[h&t.mask].getEntries()
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

// getOffsets returns potential offsets for the given hash value.
func (t *hashTable) getOffsets(h uint64) []int64 {
	e := t.getEntries(h)
	if len(e) == 0 {
		return nil
	}
	offsets := make([]int64, 0, len(e))
	base := t.hoff &^ (1<<32 - 1)
	start := t.hoff + int64(t.SliceLen()) - t.hlen
	if start < 0 {
		start = 0
	}
	for _, u := range e {
		o := base | int64(u)
		if o > t.hoff {
			o -= 1 << 32
		}
		if o < start {
			continue
		}
		offsets = append(offsets, o)
	}
	return offsets
}

// hash computes the rolling hash for p, which must have the same length as
// SliceLen() returns.
func (t *hashTable) hash(p []byte) uint64 {
	if len(p) != t.hr.Len() {
		panic("p has an incorrect length")
	}
	var h uint64
	for _, b := range p {
		h = t.hr.RollByte(b)
	}
	return h
}

// Offset returns the current head offset.
func (t *hashTable) Offset() int64 {
	return t.hoff + int64(t.SliceLen())
}

// Offsets returns all potential offsets for the byte slice. The function
// panics if p doesn't have the right length.
func (t *hashTable) Offsets(p []byte) []int64 {
	h := t.hash(p)
	offs := t.getOffsets(h)
	return offs
}
