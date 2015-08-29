package lzma2

import (
	"errors"
	"fmt"
	"io"

	"github.com/ulikunitz/xz/lzma"
)

const (
	// maximum size of compressed data in a chunk
	maxCompressed = 1 << 16
	// maximum size of uncompressed data in a chunk
	maxUncompressed = 1 << 21
)

// chunkType represents the type of an LZMA2 chunk. Note that this
// value is an internal representation and no actual encoding of a LZMA2
// chunk header.
type chunkType byte

// Possible values for the chunk type.
const (
	// end of stream
	cEOS chunkType = iota
	// uncompressed; reset dictionary
	cUD
	// uncompressed; no reset of dictionary
	cU
	// LZMA compressed; no reset
	cL
	// LZMA compressed; reset state
	cLR
	// LZMA compressed; reset state; new property value
	cLRN
	// LZMA compressed; reset state; new property value; reset dictionary
	cLRND
)

// chunkTypeStrings provide a string representation for the chunk types.
var chunkTypeStrings = [...]string{
	cEOS:  "EOS",
	cU:    "U",
	cUD:   "UD",
	cL:    "L",
	cLR:   "LR",
	cLRN:  "LRN",
	cLRND: "LRND",
}

// String returns a string representation of the chunk type.
func (c chunkType) String() string {
	if !(cEOS <= c && c <= cLRND) {
		return "unknown"
	}
	return chunkTypeStrings[c]
}

// Actual encodings for the chunk types in the value. Note that the high
// uncompressed size bits are stored in the header byte additionally.
const (
	hEOS  = 0
	hUD   = 1
	hU    = 2
	hL    = 1 << 7
	hLR   = 1<<7 | 1<<5
	hLRN  = 1<<7 | 1<<6
	hLRND = 1<<7 | 1<<6 | 1<<5
)

// errHeaderByte indicates an unsupported value for the chunk header
// byte. These bytes starts the variable-length chunk header.
var errHeaderByte = errors.New("unsupported chunk header byte")

// headerChunkType converts the header byte into a chunk type. It
// ignores the uncompressed size bits in the chunk header byte.
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

// headerLen returns the length of the LZMA2 header for a given chunk
// type.
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
	panic(fmt.Errorf("unsupported chunk type %d", c))
}

// chunkHeader represents the contents of a chunk header.
type chunkHeader struct {
	ctype        chunkType
	uncompressed uint32
	compressed   uint16
	props        lzma.Properties
}

// UnmarshalBinary reads the content of the chunk header from the data
// slice. The slice must have the correct length.
func (h *chunkHeader) UnmarshalBinary(data []byte) error {
	if len(data) == 0 {
		return errors.New("no data")
	}
	c, err := headerChunkType(data[0])
	if err != nil {
		return err
	}

	n := headerLen(c)
	if len(data) < n {
		return errors.New("incomplete data")
	}
	if len(data) > n {
		return errors.New("invalid data length")
	}

	*h = chunkHeader{ctype: c}
	if c == cEOS {
		return nil
	}

	h.uncompressed = uint32(uint16BE(data[1:3]))
	if c <= cU {
		return nil
	}
	h.uncompressed |= uint32(data[0]&^hLRND) << 16

	h.compressed = uint16BE(data[3:5])
	if c <= cLR {
		return nil
	}

	h.props = lzma.Properties(data[5])
	if h.props > lzma.MaxProperties {
		return errors.New("invalid properties")
	}
	return nil
}

// MarshalBinary encodes the chunk header value. The function checks
// whether the content of the chunk header is correct.
func (h *chunkHeader) MarshalBinary() (data []byte, err error) {
	if h.ctype > cLRND {
		return nil, errors.New("invalid chunk type")
	}
	if h.props > lzma.MaxProperties {
		return nil, errors.New("invalid properties")
	}

	data = make([]byte, headerLen(h.ctype))

	switch h.ctype {
	case cEOS:
		return data, nil
	case cUD:
		data[0] = hUD
	case cU:
		data[0] = hU
	case cL:
		data[0] = hL
	case cLR:
		data[0] = hLR
	case cLRN:
		data[0] = hLRN
	case cLRND:
		data[0] = hLRND
	}

	putUint16BE(data[1:3], uint16(h.uncompressed))
	if h.ctype <= cU {
		return data, nil
	}
	data[0] |= byte(h.uncompressed>>16) &^ hLRND

	putUint16BE(data[3:5], h.compressed)
	if h.ctype <= cLR {
		return data, nil
	}

	data[5] = byte(h.props)
	return data, nil
}

// readChunkHeader reads the chunk header from the IO reader.
func readChunkHeader(r io.Reader) (h *chunkHeader, err error) {
	p := make([]byte, 1, 6)
	if _, err = io.ReadFull(r, p); err != nil {
		return
	}
	c, err := headerChunkType(p[0])
	if err != nil {
		return
	}
	p = p[:headerLen(c)]
	if _, err = io.ReadFull(r, p[1:]); err != nil {
		return
	}
	h = new(chunkHeader)
	if err = h.UnmarshalBinary(p); err != nil {
		return nil, err
	}
	return h, nil
}

// uint16BE converts a big-endian uint16 representation to an uint16
// value.
func uint16BE(p []byte) uint16 {
	return uint16(p[0])<<8 | uint16(p[1])
}

// putUint16BE puts the big-endian uint16 presentation into the given
// slice.
func putUint16BE(p []byte, x uint16) {
	p[0] = byte(x >> 8)
	p[1] = byte(x)
}
