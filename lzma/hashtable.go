package lzma

import (
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

// slot defines the data structure for a slot in the hash table. The number of
// entries is given by slotEntries constant.
type slot struct {
	entries [slotEntries]uint32
	// start index; bit 7 set if non-empty
	i_ uint8
	// next entry to overwrite
	j_ uint8
}

// start retuns the start index of the slot
func (s *slot) start() int {
	return int(s.i_ & 0x7f)
}

// end returns the end index of the slot
func (s *slot) end() int {
	return int(s.j_)
}

// empty returns true if nothing is stored in the slot
func (s *slot) empty() bool {
	return s.i_&0x80 == 0
}

// getEntries returns all entries of a slot in the sequence that they have been
// entered.
func (s *slot) getEntries() []uint32 {
	if s.empty() {
		return nil
	}
	r := make([]uint32, 0, slotEntries)
	i, j := s.start(), s.end()
	if i >= j {
		r = append(r, s.entries[i:]...)
		r = append(r, s.entries[:j]...)
	} else {
		r = append(r, s.entries[i:j]...)
	}
	return r
}

// putEntry puts an entry into a slot.
func (s *slot) putEntry(p uint32) {
	i, j := s.start(), s.end()
	s.entries[j] = p
	jp1 := (j + 1) % slotEntries
	if j == i && !s.empty() {
		j = jp1
		i = j
	} else {
		j = jp1
	}
	s.i_ = 0x80 | uint8(i)
	s.j_ = uint8(j)
}

// hashTable stores the hash table including the rolling hash method.
type hashTable struct {
	t        []slot
	exponent int
	mask     uint64
	h        hash.Roller
}

// newHashTable creates a new hash table.
func newHashTable(exponent int, h hash.Roller) *hashTable {
	if !(minTableExponent <= exponent && exponent <= maxTableExponent) {
		panic("argument exponent out of range")
	}
	if h == nil {
		panic("argument h is nil")
	}
	slotLen := int(1) << uint(exponent)
	if slotLen <= 0 {
		panic("exponent is too large")
	}
	return &hashTable{
		t:        make([]slot, slotLen),
		exponent: exponent,
		mask:     (uint64(1) << uint(exponent)) - 1,
		h:        h}
}

// get retrieves possible values for the byte slice b. b must have at least the
// length as required by the hash function and should have the correct length.
func (t *hashTable) get(b []byte) []uint32 {
	h := hash.Hashes(t.h, b)[0]
	return t.getEntries(h)
}

// putEntry puts an entry into the hash table using the given hash.
func (t *hashTable) putEntry(h uint64, p uint32) {
	t.t[h&t.mask].putEntry(p)
}

// getEntries returns all the values that cant be found in the hash table.
func (t *hashTable) getEntries(h uint64) []uint32 {
	return t.t[h&t.mask].getEntries()
}

// put puts the hash value for a byte sequence into the hash table. b should
// have the length as supported by the rolling hash.
func (t *hashTable) put(b []byte, p uint32) {
	h := hash.Hashes(t.h, b)[0]
	t.putEntry(h, p)
}

// puts all positions for the byte sequence into the hash table. The sequences
// following the first one, will have the p value increased with the offset in
// the byte sequence.
func (t *hashTable) putAll(b []byte, p uint32) {
	hashes := hash.Hashes(t.h, b)
	for i, h := range hashes {
		t.putEntry(h, p+uint32(i))
	}
}
