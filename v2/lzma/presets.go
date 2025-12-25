package lzma

import "github.com/ulikunitz/lz"

func presetParserOptions(i int) lz.Configurator {
	_ = i
	opts := lz.GreedyParserOptions{
		MatcherOptions: &lz.GenericMatcherOptions{
			MinMatchLen: 2,
			MaxMatchLen: 273,
			MapperOptions: &lz.HashOptions{
				InputLen: 2,
				HashBits: 16,
			},
		},
	}
	return &opts
}
