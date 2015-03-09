package lzma2

import "fmt"

type DictSize byte

func FloorDictSize(size uint32) DictSize {
	panic("TODO")
}

func (s DictSize) Size() uint32 {
	if s > 40 {
		panic("invalid dictionary size")
	}
	if s == 40 {
		return 0xffffffff
	}
	m := 0x2 | (s & 1)
	exp := 11 + (s >> 1)
	r := uint32(m) << exp
	return r
}

func convert(s uint32) string {
	const (
		kib = 1024
		mib = 1024 * 1024
	)
	if s < mib {
		return fmt.Sprintf("%d KiB", s/kib)
	}
	if s < 0xffffffff {
		return fmt.Sprintf("%d MiB", s/mib)
	}
	return "4096 MiB - 1B"
}

func (s DictSize) String() string {
	return fmt.Sprintf("DictSize(%s)", convert(s.Size()))
}
