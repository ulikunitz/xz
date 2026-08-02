package lzma

import (
	"errors"

	"github.com/ulikunitz/lz"
)

func presetParserConfig(i int) lz.ParserConfig {
	_ = i
	cfg := lz.ParserConfig{
		MinMatchLen: 2,
		MaxMatchLen: 273,

		PathFinder: "greedy",
		Mapper:     "hash_2:16",
	}
	return cfg
}

// Preset returns a WriterConfig with preset parameters. Supported
// presets range from 1 to 9, from fast to slow, with increasing compression
// rates.
func Preset(n int) WriterConfig {
	if !(1 <= n && n <= 9) {
		panic(errors.New("xz: preset must be in range [1..9]"))
	}
	cfg := presets[n-1].clone()
	cfg.SetDefaults()
	return cfg
}

var presets = []WriterConfig{
	0: {
		WindowSize: 1024 << 10,
		Properties: &Properties{LC: 1, LP: 1, PB: 3},
	},
	1: {
		WindowSize: 8192 << 10,
		Properties: &Properties{LC: 0, LP: 3, PB: 4},
	},
	2: {
		WindowSize: 2048 << 10,
		Properties: &Properties{LC: 2, LP: 2, PB: 3},
	},
	3: {
		WindowSize: 8192 << 10,
		Properties: &Properties{LC: 3, LP: 1, PB: 3},
	},
	4: {
		WindowSize: 16384 << 10,
		Properties: &Properties{LC: 1, LP: 2, PB: 3},
	},
	5: {
		WindowSize: 32768 << 10,
		Properties: &Properties{LC: 0, LP: 1, PB: 2},
	},
	6: {
		WindowSize: 4096 << 10,
		Properties: &Properties{LC: 2, LP: 1, PB: 4},
	},
	7: {
		WindowSize: 65536 << 10,
		Properties: &Properties{LC: 2, LP: 1, PB: 0},
	},
	8: {
		WindowSize: 32768 << 10,
		Properties: &Properties{LC: 1, LP: 2, PB: 3},
	},
}
