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
func readUvarint(r io.ByteReader) (x uint64, err error) {
	var s uint
	for i := 0; ; i++ {
		b, err := r.ReadByte()
		if err != nil {
			return x, err
		}
		if b < 0x80 {
			if i > 9 || i == 9 && b > 1 {
				return x, overflow
			}
			return x | uint64(b)<<s, nil
		}
		x |= uint64(b&0x7f) << s
		s += 7
	}
}
