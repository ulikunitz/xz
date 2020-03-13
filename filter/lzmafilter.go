// Copyright 2014-2019 Ulrich Kunitz. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package filter

import (
	"errors"
	"fmt"
	"io"

	"github.com/ulikunitz/xz/lzma"
)

// LZMA filter constants.
const (
	LZMAFilterID  = 0x21
	LZMAFilterLen = 3
)

func NewLZMAFilter(cap int64) *LZMAFilter {
	return &LZMAFilter{dictCap: cap}
}

// LZMAFilter declares the LZMA2 filter information stored in an xz
// block header.
type LZMAFilter struct {
	dictCap int64
}

func (f LZMAFilter) GetDictCap() int64 { return f.dictCap }

// String returns a representation of the LZMA filter.
func (f LZMAFilter) String() string {
	return fmt.Sprintf("LZMA dict cap %#x", f.dictCap)
}

// id returns the ID for the LZMA2 filter.
func (f LZMAFilter) ID() uint64 { return LZMAFilterID }

// MarshalBinary converts the LZMAFilter in its encoded representation.
func (f LZMAFilter) MarshalBinary() (data []byte, err error) {
	c := lzma.EncodeDictCap(f.dictCap)
	return []byte{LZMAFilterID, 1, c}, nil
}

// UnmarshalBinary unmarshals the given data representation of the LZMA2
// filter.
func (f *LZMAFilter) UnmarshalBinary(data []byte) error {
	if len(data) != LZMAFilterLen {
		return errors.New("xz: data for LZMA2 filter has wrong length")
	}
	if data[0] != LZMAFilterID {
		return errors.New("xz: wrong LZMA2 filter id")
	}
	if data[1] != 1 {
		return errors.New("xz: wrong LZMA2 filter size")
	}
	dc, err := lzma.DecodeDictCap(data[2])
	if err != nil {
		return errors.New("xz: wrong LZMA2 dictionary size property")
	}

	f.dictCap = dc
	return nil
}

// Reader creates a new reader for the LZMA2 filter.
func (f LZMAFilter) Reader(r io.Reader, c *ReaderConfig) (fr io.Reader,
	err error) {

	config := new(lzma.Reader2Config)
	if c != nil {
		config.DictCap = c.DictCap
	}
	dc := int(f.dictCap)
	if dc < 1 {
		return nil, errors.New("xz: LZMA2 filter parameter " +
			"dictionary capacity overflow")
	}
	if dc > config.DictCap {
		config.DictCap = dc
	}

	fr, err = config.NewReader2(r)
	if err != nil {
		return nil, err
	}
	return fr, nil
}

// WriteCloser creates a io.WriteCloser for the LZMA2 filter.
func (f LZMAFilter) WriteCloser(w io.WriteCloser, c *WriterConfig,
) (fw io.WriteCloser, err error) {
	config := new(lzma.Writer2Config)
	if c != nil {
		*config = lzma.Writer2Config{
			Properties: c.Properties,
			DictCap:    c.DictCap,
			BufSize:    c.BufSize,
			Matcher:    c.Matcher,
		}
	}

	dc := int(f.dictCap)
	if dc < 1 {
		return nil, errors.New("xz: LZMA2 filter parameter " +
			"dictionary capacity overflow")
	}
	if dc > config.DictCap {
		config.DictCap = dc
	}

	fw, err = config.NewWriter2(w)
	if err != nil {
		return nil, err
	}
	return fw, nil
}

// last returns true, because an LZMA2 filter must be the last filter in
// the filter list.
func (f LZMAFilter) last() bool { return true }
