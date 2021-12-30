package lzma

import (
	"bufio"
	"errors"
	"fmt"
	"io"
)

const (
	// maximum data length of a chunk
	maxChunkLen = 1 << 16
	// maximum length of the uncompressed data in a compressed chunk
	maxChunkULen = 1 << 21
)

// chunkSelector is a byte that characterizes a chunk
type chunkSelector byte

const (
	// end of stream
	L2EOS chunkSelector = 0x00
	// raw uncompressed data, with a directory reset
	L2RD chunkSelector = 0x01
	// raw uncompressed data
	L2R chunkSelector = 0x02
	// compressed data
	L2C chunkSelector = 0x80
	// compressed data with a state reset
	L2CS chunkSelector = 0xa0
	// compressed data with a state reset and new properties
	L2CSP chunkSelector = 0xc0
	// compressed data with a state reset, new properties and a directory
	// reset
	L2CSPD chunkSelector = 0xe0
)

// chunkHeader represents the header of a chunk
type chunkHeader struct {
	selector        chunkSelector
	compressedLen   int
	uncompressedLen int
	props           Properties
}

//  nullProps represent the null value a property
var nullProps = Properties{}

// chunkState represtens a state of the chunk stream processing
type chunkState func(c chunkSelector) (state chunkState, err error)

// errInvalidSelector indicates an invalid selector for the given chunk
// processing state
var errInvalidSelector = errors.New("lzma: invalid chunk selector")

// chunkStart represets the state of the chunk processing at the begining of a
// chunk stream
func chunkStart(c chunkSelector) (state chunkState, err error) {
	switch c {
	case L2EOS:
		return chunkFinal, nil
	case L2RD:
		return chunkS1, nil
	case L2CSPD:
		return chunkS2, nil
	default:
		return nil, errInvalidSelector
	}
}

// chunkS1 represents a chunk processing state
func chunkS1(c chunkSelector) (state chunkState, err error) {
	switch c {
	case L2EOS:
		return chunkFinal, nil
	case L2R, L2RD:
		return chunkS1, nil
	case L2CSP, L2CSPD:
		return chunkS2, nil
	default:
		return nil, errInvalidSelector
	}
}

// chunkS2 represetns a chunk processing state
func chunkS2(c chunkSelector) (state chunkState, err error) {
	switch c {
	case L2EOS:
		return chunkFinal, nil
	case L2RD:
		return chunkS1, nil
	case L2R, L2C, L2CS, L2CSP, L2CSPD:
		return chunkS2, nil
	default:
		return nil, errInvalidSelector
	}
}

// chunkFinal represetnts the final chunk processing state
func chunkFinal(c chunkSelector) (state chunkState, err error) {
	return nil, errors.New("lzma: final chunk state")
}

type bufReader interface {
	io.Reader
	io.ByteReader
}

type chunkReader struct {
	chunkState chunkState
	z          bufReader
	r          reader
	u          uncompressedReader
	err        error
}

func (cr *chunkReader) init(z io.Reader, dictSize int) error {
	br, ok := z.(bufReader)
	if !ok {
		br = bufio.NewReader(z)
	}
	*cr = chunkReader{
		chunkState: chunkStart,
		z:          br,
	}
	if err := cr.r.dict.Init(dictSize, 2*dictSize); err != nil {
		return err
	}
	cr.u.dict = &cr.r.dict
	return nil
}

func (cr *chunkReader) readChunkHeader(hdr *chunkHeader) error {
	var p [5]byte
	c, err := cr.z.ReadByte()
	if err != nil {
		if err == io.EOF {
			err = io.ErrUnexpectedEOF
		}
		return err
	}
	if c < 0x80 {
		hdr.selector = chunkSelector(c)
	} else {
		hdr.selector = chunkSelector(c & 0xe0)
	}
	cr.chunkState, err = cr.chunkState(hdr.selector)
	if err != nil {
		return err
	}
	var q []byte
	switch hdr.selector {
	case L2EOS:
		cr.err = io.EOF
		return cr.err
	case L2R, L2RD:
		if _, err = io.ReadFull(cr.z, p[:2]); err != nil {
			if err == io.EOF {
				err = io.ErrUnexpectedEOF
			}
			return err
		}
		hdr.uncompressedLen = int(getBE16(p[:2])) + 1
		return nil
	case L2C, L2CS:
		q = p[:4]
	case L2CSP, L2CSPD:
		q = p[:5]
	default:
		panic("unexpected")
	}
	if _, err = io.ReadFull(cr.z, q); err != nil {
		if err == io.EOF {
			err = io.ErrUnexpectedEOF
		}
		return err
	}
	hdr.uncompressedLen = (int(c&0x1f) | int(getBE16(q[:2]))) + 1
	hdr.compressedLen = int(getBE16(q[2:4])) + 1
	if hdr.selector == L2CSP || hdr.selector == L2CSPD {
		if err = hdr.props.fromByte(q[4]); err != nil {
			return err
		}
	}

	return nil
}

func (cr *chunkReader) startNextChunk() error {
	if cr.err != nil {
		return cr.err
	}
	var (
		err error
		hdr chunkHeader
	)
	if err = cr.readChunkHeader(&hdr); err != nil {
		cr.err = err
		return err
	}

	switch hdr.selector {
	case L2RD:
		cr.r.dict.Reset()
		fallthrough
	case L2R:
		cr.u.z = &limitedReader{cr.z, int64(hdr.uncompressedLen)}
	case L2CSPD:
		cr.r.dict.Reset()
		fallthrough
	case L2CSP:
		cr.r.state.init(hdr.props)
		z := &limitedReader{cr.z, int64(hdr.compressedLen)}
		if err = cr.r.start(z, uint64(hdr.uncompressedLen)); err != nil {
			cr.err = err
			return err
		}
		cr.u.z = nil
	case L2CS:
		cr.r.state.reset()
		fallthrough
	case L2C:
		z := &limitedReader{cr.z, int64(hdr.compressedLen)}
		if err = cr.r.start(z, uint64(hdr.uncompressedLen)); err != nil {
			cr.err = err
			return err
		}
		cr.u.z = nil
	default:
		panic(fmt.Errorf("unexpected selector, %#02x", hdr.selector))
	}

	return nil
}

func (cr *chunkReader) Read(p []byte) (n int, err error) {
	if cr.err != nil {
		return 0, cr.err
	}
	for {
		var k int
		if cr.u.z != nil {
			k, err = cr.u.Read(p[n:])
		} else {
			k, err = cr.r.Read(p[n:])
		}
		n += k
		if n == len(p) {
			return n, nil
		}
		if err == io.EOF {
			if err = cr.startNextChunk(); err != nil {
				cr.err = err
				return n, err
			}
			continue
		}
		if err != nil {
			cr.err = err
			return n, err
		}
	}
}
