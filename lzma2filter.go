package xz

import (
	"errors"
	"fmt"
	"io"
	"math"
)

// lzma2Flags represents the filter properties contained in the block header.
// Currently it contains only the dictionary size.
type lzma2Flags byte

// id returns the filter id for the LZMA2 filter.
func (f lzma2Flags) id() filterID {
	return idLZMA2
}

// reserved returns the reserved bits of lzma2Flags.
func (f lzma2Flags) reserved() byte {
	return byte(f) & 0xc0
}

// dictSize() returns the dictionary size for the filter.
func (f lzma2Flags) dictSize() (n int64, err error) {
	b := byte(f & 0x3F)
	switch {
	case b > 40:
		return 0, errors.New("dictionary too large")
	case b == 40:
		return math.MaxUint32, nil
	}
	n = 2 | (int64(b) & 1)
	n <<= uint32(b>>1) + 11
	return n, nil

}

// readLZMA2Flags reads the lzma2 filter flags. It starts after the filter id
// has been read. The property size is assumed to be one.
func readLZMA2Flags(r io.Reader, propertiesSize int64) (
	f lzma2Flags, err error,
) {
	if propertiesSize != 1 {
		return 0, errors.New(
			"lzma2 filter flags: properties size must be one")
	}
	var b [1]byte
	if _, err = io.ReadFull(r, b[:1]); err != nil {
		return 0, err
	}
	f = lzma2Flags(b[0])
	if f.reserved() != 0 {
		return 0, errors.New("lzma2 filter flags: reserved bits set")
	}
	if _, err = f.dictSize(); err != nil {
		return 0, fmt.Errorf("lzma2 filter flags: %s", err)
	}
	return f, nil
}
