package lzma

import "github.com/ulikunitz/lz"

func presetParserOptions(i int) lz.Configurator {
	_ = i
	opts := &lz.GreedyParserOptions{
		BlockSize: 128 << 10,
		MatcherOptions: &lz.GenericMatcherOptions{
			MinMatchLen: 2,
			MaxMatchLen: 273,
			WindowSize:  1 << 20,
			BufferSize:  2 << 20,
			MapperOptions: &lz.HashOptions{
				InputLen: 2,
				HashBits: 16,
			},
		},
	}
	return opts
}
