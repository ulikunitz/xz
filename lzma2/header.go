package lzma2

import (
	"errors"
	"fmt"
	"io"

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

const (
	hEOS  = 0
	hUD   = 1
	hU    = 2
	hL    = 1 << 7
	hLR   = 1<<7 | 1<<5
	hLRN  = 1<<7 | 1<<6
	hLRND = 1<<7 | 1<<6 | 1<<5
)

var errHeaderByte = errors.New("unsupported chunk header byte")

func headerChunkType(h byte) (c chunkType, err error) {
	if h&hL == 0 {
		// no compression
		switch h {
		case hEOS:
			c = cEOS
		case hUD:
			c = cUD
		case hU:
			c = cU
		default:
			return 0, errHeaderByte
		}
		return
	}
	switch h & hLRND {
	case hL:
		c = cL
	case hLR:
		c = cLR
	case hLRN:
		c = cLRN
	case hLRND:
		c = cLRND
	default:
		return 0, errHeaderByte
	}
	return
}

func headerLen(c chunkType) int {
	switch c {
	case cEOS:
		return 1
	case cU, cUD:
		return 3
	case cL, cLR:
		return 5
	case cLRN, cLRND:
		return 6
	}
	panic(fmt.Sprintf("unsupported chunk type %d", c))
}

func readChunkHeader(r io.Reader) (h *chunkHeader, err error) {
	p := make([]byte, 1, 6)
	if _, err = io.ReadFull(r, p); err != nil {
		return
	}
	if c, err = headerChunkType(p[0]); err != nil {
		return
	}
	p = p[:headerLen(c)]
	if _, err = ReadFull(r, p[1:]); err != nil {
		return
	}
	panic("TODO")
}
