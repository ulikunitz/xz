// SPDX-FileCopyrightText: © 2014 Ulrich Kunitz
//
// SPDX-License-Identifier: BSD-3-Clause

package xz

import (
	"errors"
	"io"
)

// putUint32LE puts the little-endian representation of x into the first
// four bytes of p.
func putUint32LE(p []byte, x uint32) {
	p[0] = byte(x)
	p[1] = byte(x >> 8)
	p[2] = byte(x >> 16)
	p[3] = byte(x >> 24)
}

// putUint64LE puts the little-endian representation of x into the first
// eight bytes of p.
func putUint64LE(p []byte, x uint64) {
	p[0] = byte(x)
	p[1] = byte(x >> 8)
	p[2] = byte(x >> 16)
	p[3] = byte(x >> 24)
	p[4] = byte(x >> 32)
	p[5] = byte(x >> 40)
	p[6] = byte(x >> 48)
	p[7] = byte(x >> 56)
}

// uint32LE converts a little endian representation to an uint32 value.
func uint32LE(p []byte) uint32 {
	return uint32(p[0]) | uint32(p[1])<<8 | uint32(p[2])<<16 |
		uint32(p[3])<<24
}

// putUvarint puts a uvarint representation of x into the byte slice.
func putUvarint(p []byte, x uint64) int {
	i := 0
	for x >= 0x80 {
		p[i] = byte(x) | 0x80
		x >>= 7
		i++
	}
	p[i] = byte(x)
	return i + 1
}

// errOverflow indicates an overflow of the 64-bit unsigned integer.
var errOverflowU64 = errors.New("xz: uvarint overflows 64-bit unsigned integer")

// readUvarint reads a uvarint from the given byte reader.
func readUvarint(r io.ByteReader) (x uint64, n int, err error) {
	const maxUvarintLen = 10

	var s uint
	for {
		var b byte
		b, err = r.ReadByte()
		if err != nil {
			break
		}
		x |= uint64(b&0x7f) << s
		n++
		if n >= maxUvarintLen {
			if b > 1 {
				err = errOverflowU64
			}
			break
		}
		if b < 0x80 {
			break
		}
		s += 7
	}
	return x, n, err
}
