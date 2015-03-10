package lzma2

import (
	"fmt"

	"github.com/uli-go/xz/lzlib"
)

// Represents the dictionary size for the LZMA2 format.
type DictSize byte

// DictSizeCeil computes the upper ceiling DictSize for the given number of
// bytes.
func DictSizeCeil(s uint32) DictSize {
	switch {
	case s == 0xffffffff:
		return 40
	case s < 4096:
		return 0
	}
	var n int
	x := s & ((uint32(1) << 11) - 1)
	if x > 0 {
		n += 1
	}
	x = s >> 11
	k := uint(30 - lzlib.NLZ32(x))
	x >>= k
	n += (int(k) << 1) | (int(x) & 1)
	return DictSize(n)
}

// Size returns the actual size of the dictionary for a dictionary.
func (s DictSize) Size() uint32 {
	if s > 40 {
		panic("invalid dictionary size")
	}
	if s == 40 {
		return 0xffffffff
	}
	m := 0x2 | (s & 1)
	exp := 11 + (s >> 1)
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
	if s < 0xffffffff {
		return fmt.Sprintf("%d MiB", s/mib)
	}
	return "4096 MiB - 1B"
}

// String provides the dictionary size in a Kibibyte or Mebibyte
// representation.
func (s DictSize) String() string {
	return fmt.Sprintf("DictSize(%s)", convert(s.Size()))
}
