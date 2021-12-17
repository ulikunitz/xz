package lzma

import (
	"errors"
	"fmt"
	"io"
	"math"
)

const (
	// mb give the number of bytes in a megabyte.
	mb = 1 << 20
)

// minDictSize defines the minumum supported dictionary size.
const minDictSize = 1 << 12

// Properties define the properties for the LZMA and LZMA2 compression.
type Properties struct {
	LC int
	LP int
	PB int
}

// Returns the byte that encodes the properties.
func (p Properties) byte() byte {
	return (byte)((p.PB*5+p.LP)*9 + p.LC)
}

func (p Properties) Verify() error {
	if !(0 <= p.LC && p.LC <= 8) {
		return fmt.Errorf("lzma: LC out of range 0..8")
	}
	if !(0 <= p.LP && p.LP <= 4) {
		return fmt.Errorf("lzma: LP out of range 0..4")
	}
	if !(0 <= p.PB && p.PB <= 4) {
		return fmt.Errorf("lzma: PB out of range 0..4")
	}
	return nil
}

func propertiesForByte(b byte) (p Properties, err error) {
	p.LC = int(b % 9)
	b /= 9
	p.LP = int(b % 5)
	b /= 5
	p.PB = int(b)
	if p.PB > 4 {
		return Properties{}, errors.New("lzma: invalid properties byte")
	}
	return p, nil
}

type reader struct {
	rr  rawReader
	hdr header
	z   io.Reader
	err error
	pos uint64
}

var ErrEncoding = errors.New("lzma: encoding error")
var ErrUnexpectedEOS = errors.New("lzma: unexpected end of stream")

func (r *reader) Read(p []byte) (n int, err error) {
	if r.err != nil {
		return 0, r.err
	}
	delta := r.hdr.uncompressedSize - r.pos
	if uint64(len(p)) > delta {
		p = p[:delta]
	}
	n, err = r.rr.Read(p)
	if n < 0 {
		panic("negative n returned from raw_reader Read")
	}
	r.pos += uint64(n)
	// error handling seems to be complicated, but I want to make sure we
	// cover all situation.
	if err == nil {
		if r.pos == r.hdr.uncompressedSize {
			if !r.rr.rd.possiblyAtEnd() {
				r.err = ErrEncoding
				return n, r.err
			}
			r.err = io.EOF
			return n, r.err
		}
		return n, nil
	}
	if err == errEOS {
		if r.hdr.uncompressedSize == eosSize {
			if !r.rr.rd.possiblyAtEnd() {
				r.err = ErrEncoding
				return n, r.err
			}
			r.err = io.EOF
		} else {
			r.err = ErrUnexpectedEOS
		}
		return n, r.err
	}
	if err == io.EOF {
		if r.hdr.uncompressedSize == r.pos {
			if !r.rr.rd.possiblyAtEnd() {
				r.err = ErrEncoding
				return n, r.err
			}
			r.err = io.EOF
		} else {
			r.err = io.ErrUnexpectedEOF
		}
		return n, r.err
	}
	r.err = err
	return n, r.err
}

// NewReader creates a reader for LZMA-compressed streams. It doesn't use
// parallel go streams. The reader will either read the number of bytes given in
// the header or read until the EOS. It is not an error that the z reader
// doesn't stop at the LZMA stream end.
func NewReader(z io.Reader) (r io.Reader, err error) {
	nr := &reader{z: z}
	headerBuf := make([]byte, headerLen)
	if _, err = io.ReadFull(z, headerBuf); err != nil {
		if err == io.EOF {
			err = io.ErrUnexpectedEOF
		}
		return nil, err
	}
	if err = nr.hdr.parse(headerBuf); err != nil {
		return nil, err
	}
	if nr.hdr.dictSize < minDictSize {
		nr.hdr.dictSize = minDictSize
	}
	if err = nr.hdr.Verify(); err != nil {
		return nil, err
	}

	if err = nr.rr.init(z, int(nr.hdr.dictSize), nr.hdr.p); err != nil {
		return nil, err
	}

	return nr, nil
}

// WriterConfig provides configuration parameters for the LZMA writer.
type WriterConfig struct {
	Properties
	// set to true if you want LC, LP and PB actuially zero
	PropertiesInitialized bool
	DictSize              int
	MemoryBudget          int
	Effort                int
}

// NewWriter creates a single-threaded writer of LZMA files.
func NewWriter(z io.Writer) (w io.WriteCloser, err error) {
	cfg := WriterConfig{
		Properties:            Properties{LC: 3, LP: 0, PB: 2},
		PropertiesInitialized: true,
		DictSize:              8 * mb,
		MemoryBudget:          10 * mb,
		Effort:                5,
	}
	return NewWriterConfig(z, cfg)
}

// NewWriterConfig creates a new writer generating LZMA files.
func NewWriterConfig(z io.Writer, cfg WriterConfig) (w io.WriteCloser, err error) {
	panic("TODO")
}

// eosSize is used for the uncompressed size if it is unknown
const eosSize uint64 = 0xffffffffffffffff

// headerLen defines the length of an LZMA header
const headerLen = 13

// header defines an LZMA header
type header struct {
	p                Properties
	dictSize         uint32
	uncompressedSize uint64
}

func (h header) Verify() error {
	if uint64(h.dictSize) > math.MaxInt {
		return errors.New("lzma: dictSize exceed max integer")
	}
	if h.dictSize < minDictSize {
		return errors.New("lzma: dictSize is too small")
	}
	return h.p.Verify()
}

// append adds the header to the slice s.
func (h header) append(s []byte) []byte {
	var a [headerLen]byte
	a[0] = h.p.byte()
	putLE32(a[1:], h.dictSize)
	putLE64(a[5:], h.uncompressedSize)
	return append(s, a[:]...)
}

// parse parses the header from the slice x. x must have exactly header length.
func (h *header) parse(x []byte) error {
	if len(x) != headerLen {
		return errors.New("lzma: LZMA header has incorrect length")
	}
	var err error
	h.p, err = propertiesForByte(x[0])
	if err != nil {
		return err
	}
	h.dictSize = getLE32(x[1:])
	h.uncompressedSize = getLE64(x[5:])
	return nil

}
