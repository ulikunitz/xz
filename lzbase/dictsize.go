package lzbase

import (
	"fmt"
)

// Represents the dictionary size for the LZMA2 format.
type DictSize byte

const (
	// maximum code supported by the DictSize type
	maxDictSizeCode DictSize = 40
	// Shift for the mantissas 2 and 3
	dictSizeShift = 11
	// minimal supported dictionary size
	minDictSize uint32 = 2 << dictSizeShift
	// maximal supported dictionary size
	maxDictSize uint32 = 0xffffffff
)

// DictSizeCeil computes the upper ceiling DictSize for the given number of
// bytes.
func DictSizeCeil(s uint32) DictSize {
	switch {
	case s == maxDictSize:
		return maxDictSizeCode
	case s <= minDictSize:
		return 0
	}
	var n int
	x := s & ((uint32(1) << 11) - 1)
	if x > 0 {
		n += 1
	}
	x = s >> 11
	k := uint(30 - nlz32(x))
	x >>= k
	n += (int(k) << 1) | (int(x) & 1)
	return DictSize(n)
}

// Size returns the actual size of the dictionary for a dictionary.
func (s DictSize) Size() uint32 {
	if s >= maxDictSizeCode {
		if s > maxDictSizeCode {
			panic("invalid dictionary size")
		}
		return maxDictSize
	}
	m := 0x2 | (s & 1)
	exp := dictSizeShift + (s >> 1)
	r := uint32(m) << exp
	return r
}

// convert returns a string representation for a certain size.
func convert(s uint32) string {
	const (
		kib = 1024
		mib = 1024 * 1024
	)
	if s < mib {
		return fmt.Sprintf("%d KiB", s/kib)
	}
	if s < maxDictSize {
		return fmt.Sprintf("%d MiB", s/mib)
	}
	return "4096 MiB - 1B"
}

// String provides the dictionary size in a Kibibyte or Mebibyte
// representation.
func (s DictSize) String() string {
	return fmt.Sprintf("DictSize(%s)", convert(s.Size()))
}
