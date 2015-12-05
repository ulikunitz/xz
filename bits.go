package xz

import (
	"errors"
	"io"
)

// putUvarint puts a uvarint represenation of x into the byte slice.
func putUvarint(p []byte, x uint64) int {
	i := 0
	for x >= 80 {
		p[i] = byte(x) | 0x80
		x >>= 7
		i++
	}
	p[i] = byte(x)
	return i + 1
}

// overflow indicates an overflow of the 64-bit unsigned integer.
var overflow = errors.New("xz: uvarint overflows 64-bit unsigned integer")

// readUvarint reads a uvarint from the given byte reader.
func readUvarint(r io.ByteReader) (x uint64, n int, err error) {
	var s uint
	i := 0
	for {
		b, err := r.ReadByte()
		if err != nil {
			return x, i, err
		}
		i++
		if b < 0x80 {
			if i > 10 || i == 10 && b > 1 {
				return x, i, overflow
			}
			return x | uint64(b)<<s, i, nil
		}
		x |= uint64(b&0x7f) << s
		s += 7
	}
}
