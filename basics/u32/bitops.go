// Copyright 2015 Ulrich Kunitz. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// The package u32 provides basic function for the uint32 type.
package u32

/* Naming conventions follows the CodeReviewComments in the Go Wiki. */

// ntzConst is used by the functions NTZ and NLZ.
const ntzConst = 0x04d7651f

// Helper table for de Bruijn algorithm by Danny Dubé. See Henry S.
// Warren, Jr. "Hacker's Delight" section 5-1 figure 5-26.
var ntzTable = [32]int8{
	0, 1, 2, 24, 3, 19, 6, 25,
	22, 4, 20, 10, 16, 7, 12, 26,
	31, 23, 18, 5, 21, 9, 15, 11,
	30, 17, 8, 14, 29, 13, 28, 27}

// NTZ computes the number of trailing zeros for an unsigned 32-bit integer.
func NTZ(x uint32) int {
	if x == 0 {
		return 32
	}
	x = (x & -x) * ntzConst
	return int(ntzTable[x>>27])
}

// NLZ computes the number of leading zeros for an unsigned 32-bit integer.
func NLZ(x uint32) int {
	// Smear left most bit to the right
	x |= x >> 1
	x |= x >> 2
	x |= x >> 4
	x |= x >> 8
	x |= x >> 16
	// Use ntz mechanism to calculate nlz.
	x++
	if x == 0 {
		return 0
	}
	x *= ntzConst
	return 32 - int(ntzTable[x>>27])
}
