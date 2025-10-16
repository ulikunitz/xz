package lzma

import "github.com/ulikunitz/lz"

func parserPresets(n int) lz.ParserOptions {
	// TODO
	_ = n
	return &lz.GreedyParserOptions{}
}