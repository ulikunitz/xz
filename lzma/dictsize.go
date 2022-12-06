package lzma

import "errors"

// maxDictSize defines the maximum dictionary capacity supported by the
// LZMA2 dictionary capacity encoding.
const maxDictSize = 1<<32 - 1

// maxDictSizeCode defines the maximum dictionary size code.
const maxDictSizeCode = 40

// The function decodes the dictionary capacity byte, but doesn't change
// for the correct range of the given byte.
func decodeDictSize(c byte) int64 {
	return (2 | int64(c)&1) << (11 + (c>>1)&0x1f)
}

// DecodeDictSize decodes the encoded dictionary capacity. The function
// returns an error if the code is out of range.
func DecodeDictSize(c byte) (n int64, err error) {
	if c >= maxDictSizeCode {
		if c == maxDictSizeCode {
			return maxDictSize, nil
		}
		return 0, errors.New("lzma: invalid dictionary size code")
	}
	return decodeDictSize(c), nil
}

// EncodeDictSize encodes a dictionary capacity. The function returns the
// code for the capacity that is greater or equal n. If n exceeds the
// maximum support dictionary capacity, the maximum value is returned.
func EncodeDictSize(n int64) byte {
	a, b := byte(0), byte(40)
	for a < b {
		c := a + (b-a)>>1
		m := decodeDictSize(c)
		if n <= m {
			if n == m {
				return c
			}
			b = c
		} else {
			a = c + 1
		}
	}
	return a
}
