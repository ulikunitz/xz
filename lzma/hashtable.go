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

// arrayEntries gives the number of entries in the array structure.
const arrayEntries = 24

// The minTableExponent give the minimum and maximum for the table exponent.
// The minimum is somehow arbitrary but the maximum is defined by the number of
// bits in the hash value.
const (
	minTableExponent = 9
	maxTableExponent = 64
)

type array struct {
	entries [arrayEntries]uint32
	// start index
	i int8
	// next entry to overwrite
	j int8
}

func (a *array) getEntries() []uint32 {
	r := make([]uint32, 0, arrayEntries)
	if a.i > a.j {
		r = append(r, a.entries[a.i:]...)
		r = append(r, a.entries[:a.j]...)
	} else {
		r = append(r, a.entries[a.i:a.j]...)
	}
	return r
}

func (a *array) putEntry(p uint32) {
	a.entries[a.j] = p
	a.j = (a.j + 1) % arrayEntries
	if a.j == a.i {
		a.i = (a.j + 1) % arrayEntries
	}
}

type hashTable struct {
	t        []array
	exponent int
	mask     uint64
	h        hash.Roller
}

func newHashTable(exponent int, h hash.Roller) *hashTable {
	if !(minTableExponent <= exponent && exponent <= maxTableExponent) {
		panic("argument exponent out of range")
	}
	if h == nil {
		panic("argument h is nil")
	}
	arrayLen := int(1) << uint(exponent)
	if arrayLen <= 0 {
		panic("exponent is too large")
	}
	return &hashTable{
		t:        make([]array, arrayLen),
		exponent: exponent,
		mask:     (uint64(1) << uint(exponent)) - 1,
		h:        h}
}

func (t *hashTable) lookup(b []byte) []uint32 {
	h := t.h.Hashes(b)[0]
	return t.getEntries(h)
}

func (t *hashTable) putEntry(h uint64, p uint32) {
	t.t[h&t.mask].putEntry(p)
}

func (t *hashTable) getEntries(h uint64) []uint32 {
	return t.t[h&t.mask].getEntries()
}

func (t *hashTable) put(b []byte, p uint32) {
	h := t.h.Hashes(b)[0]
	t.putEntry(h, p)
}

func (t *hashTable) putAll(b []byte, p uint32) {
	hashes := t.h.Hashes(b)
	for i, h := range hashes {
		t.putEntry(h, p+uint32(i))
	}
}
