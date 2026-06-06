// SPDX-FileCopyrightText: © 2014 Ulrich Kunitz
//
// SPDX-License-Identifier: BSD-3-Clause

package xz

import (
	"errors"
	"fmt"
	"io"

	"github.com/ulikunitz/xz/v2/lzma"
)

// LZMA filter constants.
const (
	lzmaFilterID  = 0x21
	lzmaFilterLen = 3
)

// lzmaFilter declares the LZMA2 filter information stored in an xz
// block header.
type lzmaFilter struct {
	dictSize int64
}

// String returns a representation of the LZMA filter.
func (f lzmaFilter) String() string {
	return fmt.Sprintf("LZMA dict cap %#x", f.dictSize)
}

// id returns the ID for the LZMA2 filter.
func (f lzmaFilter) id() uint64 { return lzmaFilterID }

// MarshalBinary converts the lzmaFilter in its encoded representation.
func (f lzmaFilter) MarshalBinary() (data []byte, err error) {
	c := lzma.EncodeDictSize(f.dictSize)
	return []byte{lzmaFilterID, 1, c}, nil
}

// UnmarshalBinary unmarshals the given data representation of the LZMA2
// filter.
func (f *lzmaFilter) UnmarshalBinary(data []byte) error {
	if len(data) != lzmaFilterLen {
		return errors.New("xz: data for LZMA2 filter has wrong length")
	}
	if data[0] != lzmaFilterID {
		return errors.New("xz: wrong LZMA2 filter id")
	}
	if data[1] != 1 {
		return errors.New("xz: wrong LZMA2 filter size")
	}
	dc, err := lzma.DecodeDictSize(data[2])
	if err != nil {
		return errors.New("xz: wrong LZMA2 dictionary size property")
	}

	f.dictSize = dc
	return nil
}

// reader creates a new reader for the LZMA2 filter.
func (f lzmaFilter) reader(r io.Reader, c *readerConfig) (fr io.ReadCloser, err error) {

	if c == nil {
		c = &readerConfig{}
		c.setDefaults()
	}

	var cfg lzma.Reader2Options
	if c.LZMAParallel {
		cfg = lzma.Reader2Options{
			Workers:    c.Workers,
			BufferSize: c.LZMABufferSize,
		}
	} else {
		cfg = lzma.Reader2Options{
			Workers: 1,
		}
	}
	dc := int(f.dictSize)
	if dc < 1 {
		return nil, errors.New(
			"xz: LZMA2 filter parameter dictionary capacity overflow")
	}
	cfg.WindowSize = dc

	fr, err = lzma.NewReader2Options(r, cfg)
	if err != nil {
		return nil, err
	}
	return fr, nil
}

// writeCloser creates a io.WriteCloser for the LZMA2 filter.
func (f lzmaFilter) writeCloser(w io.WriteCloser, c *writerConfig,
) (fw io.WriteCloser, err error) {
	if c == nil {
		c = &writerConfig{}
		c.setDefaults()
	}

	cfg := lzma.Writer2Options{
		WindowSize:      c.WindowSize,
		BufferSize:      c.BufferSize,
		Properties:      c.Properties,
		FixedProperties: c.FixedProperties,
		ParserOptions:   c.ParserOptions,
	}
	if c.LZMAParallel {
		cfg.Workers = c.Workers
	} else {
		cfg.Workers = 1
	}

	// TODO: check
	dc := int(f.dictSize)
	if dc < 1 {
		return nil, errors.New("xz: LZMA2 filter parameter " +
			"dictionary capacity overflow")
	}
	cfg.WindowSize = dc

	fw, err = lzma.NewWriter2Options(w, cfg)
	if err != nil {
		return nil, err
	}
	return fw, nil
}

// last returns true, because an LZMA2 filter must be the last filter in
// the filter list.
func (f lzmaFilter) last() bool { return true }
