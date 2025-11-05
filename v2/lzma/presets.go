package lzma

import "github.com/ulikunitz/lz"

func parserPresets(n int) lz.ParserOptions {
	// TODO
	_ = n
	return lz.ParserOptions{
		BufferSize:     256 << 10,
		MaintainWindow: true,
	}
}
