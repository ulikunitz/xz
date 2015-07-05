// Copyright 2015 Ulrich Kunitz. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package lzma

import (
	"errors"
	"fmt"
	"io"
)

// getUint32LE reads an uint32 integer from a byte slize
func getUint32LE(b []byte) uint32 {
	x := uint32(b[3]) << 24
	x |= uint32(b[2]) << 16
	x |= uint32(b[1]) << 8
	x |= uint32(b[0])
	return x
}

// getUint64LE converts the uint64 value stored as little endian to an uint64
// value.
func getUint64LE(b []byte) uint64 {
	x := uint64(b[7]) << 56
	x |= uint64(b[6]) << 48
	x |= uint64(b[5]) << 40
	x |= uint64(b[4]) << 32
	x |= uint64(b[3]) << 24
	x |= uint64(b[2]) << 16
	x |= uint64(b[1]) << 8
	x |= uint64(b[0])
	return x
}

// putUint32LE puts an uint32 integer into a byte slice that must have at least
// a lenght of 4 bytes.
func putUint32LE(b []byte, x uint32) {
	b[0] = byte(x)
	b[1] = byte(x >> 8)
	b[2] = byte(x >> 16)
	b[3] = byte(x >> 24)
}

// putUint64LE puts the uint64 value into the byte slice as little endian
// value. The byte slice b must have at least place for 8 bytes.
func putUint64LE(b []byte, x uint64) {
	b[0] = byte(x)
	b[1] = byte(x >> 8)
	b[2] = byte(x >> 16)
	b[3] = byte(x >> 24)
	b[4] = byte(x >> 32)
	b[5] = byte(x >> 40)
	b[6] = byte(x >> 48)
	b[7] = byte(x >> 56)
}

// noHeaderLen defines the value of the length field in the LZMA header.
const noHeaderLen uint64 = 1<<64 - 1

// readHeader reads the classic LZMA header.
func readHeader(r io.Reader) (p *Parameters, err error) {
	b := make([]byte, 13)
	_, err = io.ReadFull(r, b)
	if err != nil {
		return nil, err
	}
	p = new(Parameters)
	props := Properties(b[0])
	p.LC, p.LP, p.PB = props.LC(), props.LP(), props.PB()
	p.DictSize = int64(getUint32LE(b[1:]))
	u := getUint64LE(b[5:])
	if u == noHeaderLen {
		p.Size = 0
		p.EOS = true
		p.SizeInHeader = false
	} else {
		p.Size = int64(u)
		if p.Size < 0 {
			return nil, errors.New(
				"unpack length in header not supported by" +
					" int64")
		}
		p.EOS = false
		p.SizeInHeader = true
	}

	// TODO: normalizeSizes(p)
	return p, nil
}

// writeHeader writes the header for classic LZMA files.
func writeHeader(w io.Writer, p *Parameters) error {
	var err error
	if err = p.Verify(); err != nil {
		return err
	}
	b := make([]byte, 13)
	b[0] = byte(p.Properties())
	if p.DictSize > MaxDictSize {
		return lzmaError{fmt.Sprintf(
			"DictSize %d exceeds maximum value", p.DictSize)}
	}
	putUint32LE(b[1:5], uint32(p.DictSize))
	var l uint64
	if p.SizeInHeader {
		if p.Size < 0 {
			return negError{"p.Size", p.Size}
		}
		l = uint64(p.Size)
	} else {
		l = noHeaderLen
	}
	putUint64LE(b[5:], l)
	_, err = w.Write(b)
	return err
}
