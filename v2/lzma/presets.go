package lzma

import "github.com/ulikunitz/lz"

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
