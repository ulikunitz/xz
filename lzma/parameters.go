// Copyright 2015 Ulrich Kunitz. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package lzma

import "fmt"

// Parameters contain all information required to decode or encode an LZMA
// stream.
//
// The sum of DictSize and ExtraBufSize must be less or equal MaxInt32 on
// 32-bit platforms.
type Parameters struct {
	// number of literal context bits
	LC int
	// number of literal position bits
	LP int
	// number of position bits
	PB int
	// size of the dictionary in bytes
	DictSize int64
	// size of uncompressed data in bytes
	Size int64
	// header includes unpacked size
	SizeInHeader bool
	// end-of-stream marker requested
	EOS bool
	// additional buffer size on top of dictionary size
	ExtraBufSize int64
}

// Properties returns LC, LP and PB as Properties value.
func (p *Parameters) Properties() Properties {
	props, err := NewProperties(p.LC, p.LP, p.PB)
	if err != nil {
		panic(err)
	}
	return props
}

// SetProperties sets the LC, LP and PB fields.
func (p *Parameters) SetProperties(props Properties) {
	p.LC, p.LP, p.PB = props.LC(), props.LP(), props.PB()
}

// normalizeDictSize provides a correct value for the dictionary size.
// If the dictionary size is 0 the default value is used. Values less
// then the minimum dictionary size are corrected.
func (p *Parameters) normalizeDictSize() {
	if p.DictSize == 0 {
		p.DictSize = Default.DictSize
	}
	if p.DictSize < MinDictSize {
		p.DictSize = MinDictSize
	}
}

// normalizeReaderExtraBufSize provides a correct value for the extra
// buffer size for the LZMA reader. The ExtraBufSize is set to 0 if
// negative.
func (p *Parameters) normalizeReaderExtraBufSize() {
	if p.ExtraBufSize < 0 {
		p.ExtraBufSize = 0
	}
}

// normalizeWriterExtraBufSize provides a correct value for the extra
// buffer size for an LZMA writer. It is set to 4096 if the extra buffer
// size is less then zero.
func (p *Parameters) normalizeWriterExtraBufSize() {
	if p.ExtraBufSize <= 0 {
		p.ExtraBufSize = 4096
	}
}

func (p *Parameters) normalizeReaderSizes() {
	p.normalizeDictSize()
	p.normalizeReaderExtraBufSize()
}

func (p *Parameters) normalizeWriterSizes() {
	p.normalizeDictSize()
	p.normalizeWriterExtraBufSize()
}

// Verify checks parameters for errors.
func (p *Parameters) Verify() error {
	if p == nil {
		return lzmaError{"parameters must be non-nil"}
	}
	if err := verifyProperties(p.LC, p.LP, p.PB); err != nil {
		return err
	}
	if !(MinDictSize <= p.DictSize && p.DictSize <= MaxDictSize) {
		return rangeError{"DictSize", p.DictSize}
	}
	if p.DictSize != int64(int(p.DictSize)) {
		return lzmaError{fmt.Sprintf("DictSize %d too large for int", p.DictSize)}
	}
	if p.Size < 0 {
		return negError{"Size", p.Size}
	}
	if p.ExtraBufSize < 0 {
		return negError{"ExtraBufSize", p.ExtraBufSize}
	}
	bufSize := p.DictSize + p.ExtraBufSize
	if bufSize != int64(int(bufSize)) {
		return lzmaError{"buffer size too large for int"}
	}
	return nil
}

// Default defines standard parameters.
//
// Use normalizeWriterExtraBufSize to set extra buf size to a reasonable
// value.
var Default = Parameters{
	LC:       3,
	LP:       0,
	PB:       2,
	DictSize: 8 * 1024 * 1024,
}
