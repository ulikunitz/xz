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

// lzmaFilter represents the LZMA2 filter information stored in an XZ
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

// MarshalBinary converts the lzmaFilter into its encoded representation.
func (f lzmaFilter) MarshalBinary() (data []byte, err error) {
	c := lzma.EncodeWindowSize(f.dictSize)
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
	dc, err := lzma.DecodeWindowSize(data[2])
	if err != nil {
		return errors.New("xz: wrong LZMA2 dictionary size property")
	}

	f.dictSize = dc
	return nil
}

// reader creates a new reader for the LZMA2 filter.
func (f lzmaFilter) reader(r io.Reader, c *ReaderConfig) (fr io.ReadCloser, err error) {

	if c == nil {
		c = &ReaderConfig{}
		c.SetDefaults()
	}

	var cfg lzma.Reader2Config
	if c.LZMAParallel {
		cfg = lzma.Reader2Config{
			Workers:    c.Workers,
			BufferSize: c.LZMABufferSize,
		}
	} else {
		cfg = lzma.Reader2Config{
			Workers: 1,
		}
	}
	dc := int(f.dictSize)
	if dc < 1 {
		return nil, errors.New(
			"xz: LZMA2 filter parameter dictionary capacity overflow")
	}
	cfg.WindowSize = dc

	fr, err = lzma.NewReader2Config(r, cfg)
	if err != nil {
		return nil, err
	}
	return fr, nil
}

// writeCloser creates a io.WriteCloser for the LZMA2 filter.
func (f lzmaFilter) writeCloser(w io.WriteCloser, c *WriterConfig,
) (fw io.WriteCloser, err error) {
	if c == nil {
		c = &WriterConfig{}
		c.SetDefaults()
	}

	// TODO
	cfg := lzma.Writer2Config{
		WindowSize: c.WindowSize,
		BufferSize: c.BufferSize,
		Properties: c.Properties,
		PathFinder: c.PathFinder,
		Mapper:     c.Mapper,
	}
	if c.LZMAParallel {
		cfg.Workers = c.Workers
	} else {
		cfg.Workers = 1
	}

	dc := int(f.dictSize)
	if dc < 1 {
		return nil, errors.New("xz: LZMA2 filter parameter " +
			"dictionary capacity overflow")
	}
	cfg.WindowSize = dc

	fw, err = lzma.NewWriter2Config(w, cfg)
	if err != nil {
		return nil, err
	}
	return fw, nil
}

// last returns true, because an LZMA2 filter must be the last filter in
// the filter list.
func (f lzmaFilter) last() bool { return true }
