// Copyright 2015 Ulrich Kunitz. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package lzma

import (
	"errors"
	"fmt"
	"io"
)

// Parameters provides the Parameters of an LZMA reader and LZMA writer.
type Parameters struct {
	Properties Properties
	DictCap    int
	// uncompressed size; negative value if no size given
	Size int64
	// minimum size for the buffer
	BufSize   int
	EOSMarker bool
}

// Default provides the default parameters for the LZMA writer.
var Default = Parameters{
	Properties: Properties{3, 0, 2},
	DictCap:    8 * 1024 * 1024,
	Size:       -1,
}

// normalizeReader normalizes the parameters for the LZMA reader.
func (p *Parameters) normalizeReader() {
	if p.DictCap < MinDictCap {
		p.DictCap = MinDictCap
	}
	if p.Size < 0 {
		p.EOSMarker = true
	}
	if p.BufSize <= 0 {
		p.BufSize = p.DictCap
	}
}

// readHeader reads the classic LZMA header.
func readHeader(r io.Reader) (p *Parameters, err error) {
	b := make([]byte, 13)
	_, err = io.ReadFull(r, b)
	if err != nil {
		return nil, err
	}
	props, err := PropertiesForCode(b[0])
	if err != nil {
		return nil, err
	}
	p = &Parameters{
		Properties: props,
		Size:       -1,
	}
	p.DictCap = int(uint32LE(b[1:]))
	if p.DictCap < 0 {
		return nil, errors.New(
			"LZMA header: dictionary capacity exceeds maximum " +
				"integer")
	}
	p.BufSize = p.DictCap
	u := uint64LE(b[5:])
	if u == noHeaderSize {
		p.EOSMarker = true
	} else {
		p.Size = int64(u)
		if p.Size < 0 {
			return nil, errors.New(
				"LZMA header: uncompressed length in header " +
					" out of int64 range")
		}
	}
	return p, nil
}

// writeHeader writes the header for classic LZMA files.
func writeHeader(w io.Writer, p *Parameters) error {
	if err := p.Properties.Verify(); err != nil {
		return err
	}
	b := make([]byte, 13)
	b[0] = p.Properties.Code()
	if !(0 <= p.DictCap && p.DictCap <= MaxDictCap) {
		return fmt.Errorf("write LZMA header: DictCap %d out of range",
			p.DictCap)
	}
	putUint32LE(b[1:5], uint32(p.DictCap))
	var l uint64
	if p.Size >= 0 {
		l = uint64(p.Size)
	} else {
		l = noHeaderSize
	}
	putUint64LE(b[5:], l)
	_, err := w.Write(b)
	return err
}
