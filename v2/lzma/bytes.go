// SPDX-FileCopyrightText: © 2014 Ulrich Kunitz
//
// SPDX-License-Identifier: BSD-3-Clause

package lzma

// putLE32 writes a uint32 value into the slice p using little-endian encoding.
// The p slice must have at least length four.
func putLE32(p []byte, x uint32) {
	_ = p[3]
	p[0] = byte(x)
	p[1] = byte(x >> 8)
	p[2] = byte(x >> 16)
	p[3] = byte(x >> 24)
}

/* TODO
// _getLE64 loads a uint64 value from the p field. This function will be inlined
// and compiled into a simple move on little-endian 64 bit architectures.
//
// If p is too small the function will panic.
func _getLE64(p []byte) uint64 {
	_ = p[7]
	return uint64(p[0]) | uint64(p[1])<<8 | uint64(p[2])<<16 |
		uint64(p[3])<<24 | uint64(p[4])<<32 | uint64(p[5])<<40 |
		uint64(p[6])<<48 | uint64(p[7])<<56
}
*/

// getLE32 reads a uint32 value from the slice p. Slice p must have at least
// length 4.
func getLE32(p []byte) uint32 {
	_ = p[3]
	var x uint32
	x = uint32(p[0])
	x |= uint32(p[1]) << 8
	x |= uint32(p[2]) << 16
	x |= uint32(p[3]) << 24
	return x
}

/* TODO
// _getLE32 loads a uint32 value from the p field. This function will be inlined
// and compiled into a simple move on little-endian architectures.
//
// If p is too small the function will panic.
func _getLE32(p []byte) uint32 {
	_ = p[3]
	return uint32(p[0]) | uint32(p[1])<<8 | uint32(p[2])<<16 |
		uint32(p[3])<<24
}
*/

// putLE64 writes a uint64 value into the slice p using little endian encoding.
// The length of slice p must be at least 8 bytes.
func putLE64(p []byte, x uint64) {
	_ = p[7]
	p[0] = byte(x)
	p[1] = byte(x >> 8)
	p[2] = byte(x >> 16)
	p[3] = byte(x >> 24)
	p[4] = byte(x >> 32)
	p[5] = byte(x >> 40)
	p[6] = byte(x >> 48)
	p[7] = byte(x >> 56)
}

// getLE64 reads a uint64 value from the slice p using little endian encoding.
// The length of p must be at least 8 bytes.
func getLE64(p []byte) uint64 {
	_ = p[7]
	var x uint64
	x = uint64(p[0])
	x |= uint64(p[1]) << 8
	x |= uint64(p[2]) << 16
	x |= uint64(p[3]) << 24
	x |= uint64(p[4]) << 32
	x |= uint64(p[5]) << 40
	x |= uint64(p[6]) << 48
	x |= uint64(p[7]) << 56
	return x
}

// getBE16 reads a uin16 value from slice p using big endian encoding. The
// length of p must be at least 2 bytes.
func getBE16(p []byte) uint16 {
	_ = p[1]
	return uint16(p[0])<<8 | uint16(p[1])
}

// putBE16 writes the value x into p using big endian encoding. The slice p must
// have space for at least two bytes.
func putBE16(p []byte, x uint16) {
	_ = p[1]
	p[0] = uint8(x >> 8)
	p[1] = uint8(x)
}

/* TODO
// lcp computes the length of the longest common prefix between p and q.
func lcp(p, q []byte) int {
	if len(q) > len(p) {
		p, q = q, p
	}
	n := 0
	for len(q) >= 8 {
		x := _getLE64(p) ^ _getLE64(q)
		k := bits.TrailingZeros64(x) >> 3
		n += k
		if k < 8 {
			return n
		}
		q = q[8:]
		p = p[8:]
	}
	if len(q) >= 4 {
		x := _getLE32(p) ^ _getLE32(q)
		k := bits.TrailingZeros32(x) >> 3
		n += k
		if k < 4 {
			return n
		}
		q = q[4:]
		p = p[4:]
	}
	for i, b := range q {
		if p[i] != b {
			break
		}
		n++
	}
	return n
}
*/
