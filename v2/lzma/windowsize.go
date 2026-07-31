// SPDX-FileCopyrightText: © 2014 Ulrich Kunitz
//
// SPDX-License-Identifier: BSD-3-Clause

package lzma

import "errors"

// maxWindowSize defines the maximum dictionary capacity supported by the
// LZMA2 dictionary capacity encoding.
const maxWindowSize = 1<<32 - 1

// maxDictSizeCode defines the maximum dictionary size code.
const maxWindowSizeCode = 40

// decodeWindowSize decodes the encoded dictionary size.
func decodeWindowSize(c byte) int64 {
	return (2 | int64(c)&1) << (11 + (c>>1)&0x1f)
}

// DecodeWindowSize decodes the encoded dictionary size. The function
// returns an error if the code is out of range.
func DecodeWindowSize(c byte) (n int64, err error) {
	if c >= maxWindowSizeCode {
		if c == maxWindowSizeCode {
			return maxWindowSize, nil
		}
		return 0, errors.New("lzma: invalid dictionary size code")
	}
	return decodeWindowSize(c), nil
}

// EncodeWindowSize encodes a dictionary capacity. The function returns the
// code for the capacity that is greater or equal n. If n exceeds the
// maximum support dictionary capacity, the maximum value is returned.
func EncodeWindowSize(n int64) byte {
	a, b := byte(0), byte(maxWindowSizeCode)
	for a < b {
		c := a + (b-a)>>1
		m := decodeWindowSize(c)
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
