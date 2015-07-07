package lzma2

import (
	"github.com/uli-go/xz/lzma"
)

type chunkType byte

const (
	cEOS chunkType = iota
	cU
	cUD
	cL
	cLR
	cLRN
	cLRND
)

var chunkTypeStrings = [...]string{
	cEOS:  "EOS",
	cU:    "U",
	cUD:   "UD",
	cL:    "L",
	cLR:   "LR",
	cLRN:  "LRN",
	cLRND: "LRND",
}

func (c chunkType) String() string {
	if !(cEOS <= c && c <= cLRND) {
		return "unknown"
	}
	return chunkTypeStrings[c]
}

type chunkHeader struct {
	ctype    chunkType
	unpacked uint32
	packed   uint16
	props    lzma.Properties
}

func headerChunkType(h byte) (c chunkType, err error) {
	panic("TODO")
}
