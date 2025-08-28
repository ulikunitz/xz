// SPDX-FileCopyrightText: © 2014 Ulrich Kunitz
//
// SPDX-License-Identifier: BSD-3-Clause

package lzma

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"math"

	"github.com/ulikunitz/lz"
)

// ReaderConfig stores the parameters for the reader of the classic LZMA
// format.
type ReaderConfig struct {
	// Since v0.5.14 this parameter sets an upper limit for a .lzma file's
	// dictionary size. This helps to mitigate problems with mangled
	// headers.
	DictCap int
}

// setDefaults converts the zero values of the configuration to the default values.
func (c *ReaderConfig) setDefaults() {
	if c.DictCap == 0 {
		// set an upper limit of 2 GB for dictionary capacity to address
		// the zero prefix security issue.
		c.DictCap = 1 << 31
		// original: c.DictCap = 8 * 1024 * 1024
	}
}

// Verify checks the reader configuration for errors. Zero values will
// be replaced by default values.
func (c *ReaderConfig) Verify() error {
	if !(minWindowSize <= c.DictCap && int64(c.DictCap) <= maxDictSize) {
		return errors.New("lzma: dictionary capacity is out of range")
	}
	return nil
}

// Reader provides a reader for LZMA files or streams.
//
// # Security concerns
//
// Note that LZMA format doesn't support a magic marker in the header. So
// [NewReader] cannot determine whether it reads the actual header. For instance
// the LZMA stream might have a zero byte in front of the reader, leading to
// larger dictionary sizes and file sizes. The code will detect later that there
// are problems with the stream, but the dictionary has already been allocated
// and this might consume a lot of memory.
//
// Version 0.5.14 introduces built-in mitigations:
//
//   - The [ReaderConfig] DictCap field is now interpreted as a limit for the
//     dictionary size.
//   - The default is 2 Gigabytes (2^31 bytes).
//   - Users can check with the [Reader.Header] method what the actual values are in
//     their LZMA files and set a smaller limit using [ReaderConfig].
//   - The dictionary size doesn't exceed the larger of the file size and
//     the minimum dictionary size. This is another measure to prevent huge
//     memory allocations for the dictionary.
//   - The code supports stream sizes only up to a pebibyte (1024^5).
type Reader struct {
	decoder
	// size < 0 means we wait for EOS
	size int64
	err  error

	hdr Header
}

// EOSSize marks a stream that requires the EOS marker to identify the end of
// the stream. It is used by [NewRawReader].
const EOSSize uint64 = 1<<64 - 1

// NewRawReader returns a reader that can read a LZMA stream. For a stream with
// an EOS marker use [EOSSize] for uncompressedSize. The dictSize must be
// positive (>=0).
func NewRawReader(z io.Reader, hdr Header) (r *Reader, err error) {
	if err = hdr.Verify(); err != nil {
		return nil, err
	}
	rr := new(Reader)
	if err = rr.init(z, hdr); err != nil {
		return nil, err
	}
	return rr, nil
}

// minWindowSize defines the minimum supported dictionary size.
const minWindowSize = 1 << 12

// headerLen defines the length of an LZMA header
const headerLen = 13

// Header defines the parameters for the LZMA method
type Header struct {
	Properties       Properties
	DictSize         uint32
	uncompressedSize uint64
}

// Verify checks the parameters for correctness.
func (h Header) Verify() error {
	if uint64(h.DictSize) > math.MaxInt {
		return errors.New("lzma: dictSize exceed max integer")
	}
	if h.DictSize < minWindowSize {
		return errors.New("lzma: dictSize is too small")
	}
	return h.Properties.Verify()
}

// AppendBinary adds the header to the slice s.
func (h Header) AppendBinary(p []byte) (r []byte, err error) {
	var a [headerLen]byte
	a[0] = h.Properties.byte()
	putLE32(a[1:], h.DictSize)
	putLE64(a[5:], h.uncompressedSize)
	return append(p, a[:]...), nil
}

// UnmarshalBinary parses the header from the slice x. x must have exactly header length.
func (h *Header) UnmarshalBinary(x []byte) error {
	if len(x) != headerLen {
		return errors.New("lzma: LZMA header has incorrect length")
	}
	var err error
	if err = h.Properties.fromByte(x[0]); err != nil {
		return err
	}
	h.DictSize = getLE32(x[1:])
	h.uncompressedSize = getLE64(x[5:])
	return nil
}

func (r *Reader) Header() Header { return r.hdr }

// We support only files not larger than 1 << 50 bytes (a pebibyte, 1024^5).
const maxStreamSize = 1 << 50

// ErrDictSize reports about an error of the dictionary size.
type ErrDictSize struct {
	ConfigDictCap  int
	HeaderDictSize uint32
	Message        string
}

// Error returns the error message.
func (e *ErrDictSize) Error() string {
	return e.Message
}

func newErrDictSize(messageformat string,
	configDictCap int, headerDictSize uint32,
	args ...interface{}) *ErrDictSize {
	newArgs := make([]interface{}, len(args)+2)
	newArgs[0] = configDictCap
	newArgs[1] = headerDictSize
	copy(newArgs[2:], args)
	return &ErrDictSize{
		ConfigDictCap:  configDictCap,
		HeaderDictSize: headerDictSize,
		Message:        fmt.Sprintf(messageformat, newArgs...),
	}
}

// NewReader creates a new reader for an LZMA stream.
func NewReader(z io.Reader) (r *Reader, err error) {
	return NewReaderConfig(z, ReaderConfig{})
}

// NewReaderConfig creates a new reader for the LZMA stream.
func NewReaderConfig(z io.Reader, cfg ReaderConfig) (r *Reader, err error) {
	cfg.setDefaults()
	if err = cfg.Verify(); err != nil {
		return nil, err
	}

	var p = make([]byte, headerLen)
	if _, err = io.ReadFull(z, p); err != nil {
		return nil, err
	}
	var hdr Header
	if err = hdr.UnmarshalBinary(p); err != nil {
		return nil, err
	}
	hdrOrig := hdr

	if int64(cfg.DictCap) < int64(hdr.DictSize) {
		return nil, newErrDictSize(
			"lzma: header dictionary size %[2]d exceeds configured dictionary capacity %[1]d",
			cfg.DictCap, hdr.DictSize,
		)
	}
	// Mitigation for CVE-2025-58058
	if uint64(hdr.DictSize) > hdr.uncompressedSize {
		hdr.DictSize = uint32(hdr.uncompressedSize)
	}
	// The LZMA specification says that if the dictionary size in the header
	// is less than 4096 it must be set to 4096. See pull request
	// https://github.com/ulikunitz/xz/pull/52
	// TODO: depending on the discussion we might even need a way to
	// override the header.
	if hdr.DictSize < minWindowSize {
		hdr.DictSize = minWindowSize
	}
	if err = hdr.Verify(); err != nil {
		return nil, err
	}

	if uint64(hdr.DictSize) > math.MaxInt {
		return nil, errors.New("lzma: dictSize too large")
	}

	rr := new(Reader)
	err = rr.init(z, hdr)
	if err != nil {
		return nil, err
	}
	rr.hdr = hdrOrig

	return rr, nil
}

// init initializes the reader.
func (r *Reader) init(z io.Reader, hdr Header) error {

	if err := r.buffer.Init(lz.DecoderConfig{WindowSize: int(hdr.DictSize)}); err != nil {
		return err
	}

	r.state.init(hdr.Properties)

	switch {
	case hdr.uncompressedSize == EOSSize:
		r.size = -1
	case hdr.uncompressedSize <= math.MaxInt64:
		r.size = int64(hdr.uncompressedSize)
	default:
		return errors.New("lzma: size overflow")
	}
	if r.size > maxStreamSize {
		return errors.New("lzma: stream size too large")
	}

	br, ok := z.(io.ByteReader)
	if !ok {
		br = bufio.NewReader(z)
	}

	if err := r.rd.init(br); err != nil {
		return err
	}

	switch {
	case hdr.uncompressedSize == EOSSize:
		r.size = -1
	case hdr.uncompressedSize <= math.MaxInt64:
		r.size = int64(hdr.uncompressedSize)
	default:
		return errors.New("lzma: size overflow")
	}

	r.err = nil
	r.hdr = hdr
	return nil
}

// errEOS informs that an EOS marker has been found
var errEOS = errors.New("EOS marker")

// Distance for EOS marker
const eosDist = 1<<32 - 1

// ErrEncoding reports an encoding error
var ErrEncoding = errors.New("lzma: wrong encoding")

// fillBuffer refills the buffer.
func (r *Reader) fillBuffer() error {
	for {
		if a := r.buffer.BufferSize - len(r.buffer.Data); a < maxMatchLen {
			break
		}
		seq, err := r.readSeq()
		if err != nil {
			s := r.size
			switch err {
			case errEOS:
				if r.rd.possiblyAtEnd() && (s < 0 || s == r.buffer.Off) {
					err = io.EOF
				}
			case io.EOF:
				if !r.rd.possiblyAtEnd() || s != r.buffer.Off {
					err = io.ErrUnexpectedEOF
				}
			}
			return err
		}
		if seq.MatchLen == 0 {
			if err = r.buffer.WriteByte(byte(seq.Aux)); err != nil {
				panic(err)
			}
		} else {
			_, err = r.buffer.WriteMatch(seq.MatchLen, seq.Offset)
			if err != nil {
				return err
			}
		}
		if r.size == r.buffer.Off {
			err = io.EOF
			if !r.rd.possiblyAtEnd() {
				_, serr := r.readSeq()
				if serr != errEOS || !r.rd.possiblyAtEnd() {
					err = ErrEncoding
				}
			}
			return err
		}
	}
	return nil
}

// Read reads data from the dictionary and refills it if needed.
func (r *Reader) Read(p []byte) (n int, err error) {
	k := len(r.buffer.Data) - r.buffer.R
	if r.err != nil && k == 0 {
		return 0, r.err
	}
	for {
		// Read from a dictionary never returns an error
		k, _ := r.buffer.Read(p[n:])
		n += k
		if n == len(p) {
			return n, nil
		}
		if r.err != nil {
			return n, r.err
		}
		if err = r.fillBuffer(); err != nil {
			r.err = err
			k := len(r.buffer.Data) - r.buffer.R
			if k > 0 {
				continue
			}
			return n, err
		}
	}
}
