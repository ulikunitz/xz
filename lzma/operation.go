// Copyright 2014-2022 Ulrich Kunitz. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package lzma

import (
	"fmt"
	"unicode"
)

// operation represents an operation on the dictionary during encoding or
// decoding. They are either matches or literals (or are invalid), packed into
// 64 bits (16 4-bit nibbles) as:
//
//	most significant bits                      least significant bits
//	+---+---+---+---+---+---+---+---+---+---+---+---+---+---+---+---+
//	|TYP|LITERAL|     LENGTH    |              DISTANCE             |
//	+---+---+---+---+---+---+---+---+---+---+---+---+---+---+---+---+
//	   60  56  52  48  44  40  36  32  28  24  20  16  12   8   4   0
//
// TYP (4 bits) is 0x8 for literal operations and 0x0 for match operations.
//
// LITERAL (8 bits) is the literal byte, unused (0) for match operations.
//
// LENGTH (16 bits, since maxMatchLen = 273) is the match length or 1 for
// literal operations.
//
// DISTANCE (36 bits, since maxDistance = 1 << 32) is the match distance,
// unused (0) for literal operations.
//
// The zero value is an invalid operation. Valid operations have positive
// length().
type operation uint64

func (o operation) distance() int64 { return int64(o & 0xFFFFFFFFF) }
func (o operation) length() int     { return int(uint16(o >> 36)) }
func (o operation) literal() byte   { return byte(o >> 52) }
func (o operation) isLiteral() bool { return int64(o) < 0 }

func (o operation) String() string {
	if o.isLiteral() {
		c := o.literal()
		if !unicode.IsPrint(rune(c)) {
			c = '.'
		}
		return fmt.Sprintf("L{%c/%02x}", c, c)
	}

	return fmt.Sprintf("M{%d,%d}", o.distance(), o.length())
}

func makeMatchOp(distance int64, length int) operation {
	return operation(distance&0xFFFFFFFFF) | (operation(length&0xFFFF) << 36)
}

func makeLitOp(b byte) operation {
	return 0x8000001000000000 | (operation(b) << 52)
}
