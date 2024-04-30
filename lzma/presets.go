// SPDX-FileCopyrightText: © 2014 Ulrich Kunitz
//
// SPDX-License-Identifier: BSD-3-Clause

package lzma

import (
	"errors"

	"github.com/ulikunitz/lz"
)

// Preset returns a WriterConfig with preset parameters. Supported
// presets are ranging from 1 to 9 from fast to slow with increasing
// compression rate.
func Preset(n int) WriterConfig {
	if !(1 <= n && n <= 9) {
		panic(errors.New("xz: preset must be in range [1..9]"))
	}
	cfg := presets[n-1]
	return cfg.Clone()
}

// presets contain the predefined tickets. Don't use directly to prevent
// modification.
var presets = []WriterConfig{
	0: {
		WindowSize: 1024 << 10,
		Properties: lzma.Properties{LC: 1, LP: 1, PB: 3},
		ParserConfig: &lz.HPConfig{
			BlockSize: 128 << 10,
			InputLen:  4,
			HashBits:  14,
		},
	},
	1: {
		WindowSize: 8192 << 10,
		Properties: lzma.Properties{LC: 0, LP: 3, PB: 4},
		ParserConfig: &lz.BHPConfig{
			BlockSize: 256 << 10,
			InputLen:  6,
			HashBits:  18,
		},
	},
	2: {
		WindowSize: 2048 << 10,
		Properties: lzma.Properties{LC: 2, LP: 2, PB: 3},
		ParserConfig: &lz.BDHPConfig{
			BlockSize: 32 << 10,
			InputLen1: 6,
			HashBits1: 20,
			InputLen2: 7,
			HashBits2: 8,
		},
	},
	3: {
		WindowSize: 8192 << 10,
		Properties: lzma.Properties{LC: 3, LP: 1, PB: 3},
		ParserConfig: &lz.BUPConfig{
			BlockSize:  256 << 10,
			InputLen:   5,
			HashBits:   14,
			BucketSize: 14,
		},
	},
	4: {
		WindowSize: 16384 << 10,
		Properties: lzma.Properties{LC: 1, LP: 2, PB: 3},
		ParserConfig: &lz.BUPConfig{
			BlockSize:  128 << 10,
			InputLen:   6,
			HashBits:   15,
			BucketSize: 15,
		},
	},
	5: {
		WindowSize: 32768 << 10,
		Properties: lzma.Properties{LC: 0, LP: 1, PB: 2},
		ParserConfig: &lz.BUPConfig{
			BlockSize:  64 << 10,
			InputLen:   6,
			HashBits:   18,
			BucketSize: 18,
		},
	},
	6: {
		WindowSize: 4096 << 10,
		Properties: lzma.Properties{LC: 2, LP: 1, PB: 4},
		ParserConfig: &lz.BUPConfig{
			BlockSize:  256 << 10,
			InputLen:   6,
			HashBits:   20,
			BucketSize: 20,
		},
	},
	7: {
		WindowSize: 65536 << 10,
		Properties: lzma.Properties{LC: 2, LP: 1, PB: 0},
		ParserConfig: &lz.BUPConfig{
			BlockSize:  128 << 10,
			InputLen:   7,
			HashBits:   20,
			BucketSize: 20,
		},
	},
	8: {
		WindowSize: 32768 << 10,
		Properties: lzma.Properties{LC: 1, LP: 2, PB: 3},
		ParserConfig: &lz.OSAPConfig{
			BlockSize:   256 << 10,
			MinMatchLen: 4,
		},
	},
}
