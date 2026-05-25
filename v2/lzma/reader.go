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

// readerConfig stores the parameters for the reader of the classic LZMA
// format.
type readerConfig struct {
	// Since v0.5.14 this parameter sets an upper limit for a .lzma file's
	// dictionary size. This helps to mitigate problems with mangled
	// headers.
	DictCap int
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
//   - The [readerConfig] DictCap field is now interpreted as a limit for the
//     dictionary size.
//   - The default is 2 Gigabytes (2^31 bytes).
//   - Users can check with the [Reader.Header] method what the actual values are in
//     their LZMA files and set a smaller limit using [readerConfig].
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
	if err = hdr.verify(); err != nil {
		return nil, err
	}
	rr := new(Reader)
	if err = rr.init(z, hdr); err != nil {
		return nil, err
	}
	return rr, nil
}

// minDictSize defines the minimum supported dictionary size.
const minDictSize = 1 << 12

// headerLen defines the length of an LZMA header
const headerLen = 13

// Header defines the parameters for the LZMA method
type Header struct {
	Properties       Properties
	DictSize         uint32
	uncompressedSize uint64
}

// Verify checks the parameters for correctness.
func (h Header) verify() error {
	if uint64(h.DictSize) > math.MaxInt {
		return errors.New("lzma: dictSize exceed max integer")
	}
	return h.Properties.verify()
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

// Header returns the LZMA header that was used to initialize the Reader.
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

func newErrDictSize(messageFormat string,
	configDictCap int, headerDictSize uint32,
	args ...any) *ErrDictSize {
	newArgs := make([]any, len(args)+2)
	newArgs[0] = configDictCap
	newArgs[1] = headerDictSize
	copy(newArgs[2:], args)
	return &ErrDictSize{
		ConfigDictCap:  configDictCap,
		HeaderDictSize: headerDictSize,
		Message:        fmt.Sprintf(messageFormat, newArgs...),
	}
}

// ReaderOption provides the interface for options that can be used to
// configure a LZMA reader.
type ReaderOption interface {
	updateReaderConfig(*readerConfig) error
}

type dictCapOption int

func (o dictCapOption) updateReaderConfig(c *readerConfig) error {
	if !(minDictSize <= int(o) && int64(o) <= maxDictSize) {
		return fmt.Errorf("lzma: dictionary capacity is out of range [%d, %d]", minDictSize, maxDictSize)
	}
	c.DictCap = int(o)
	return nil
}

// WithDictCap sets an upper limit for the dictionary size accepted from the
// LZMA header. Use this to mitigate excessive memory allocation when
// reading possibly malformed or malicious streams.
func WithDictCap(dictCap int) ReaderOption {
	return dictCapOption(dictCap)
}

// NewReader creates a new reader for an LZMA stream.
func NewReader(z io.Reader, options ...ReaderOption) (r *Reader, err error) {
	cfg := readerConfig{
		DictCap: (1 << 31) - 1,
	}

	for _, opt := range options {
		if err = opt.updateReaderConfig(&cfg); err != nil {
			return nil, err
		}
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
	if hdr.DictSize < minDictSize {
		hdr.DictSize = minDictSize
	}
	if err = hdr.verify(); err != nil {
		return nil, err
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
	if int64(hdr.DictSize) > math.MaxInt {
		return errors.New("lzma: dictSize too large")
	}

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

	winSize := max(int(hdr.DictSize), minDictSize)
	const maxWinSize = min(maxDictSize, math.MaxInt-maxMatchLen)
	if winSize > maxWinSize {
		return fmt.Errorf(
			"lzma: dictionary size must be at most %d",
			maxWinSize)
	}
	bufSize := 2 * winSize
	if bufSize < 0 {
		bufSize = math.MaxInt
	}
	if r.size >= 0 && bufSize > int(r.size) {
		bufSize = int(r.size)
	}
	bufSize = max(bufSize, winSize+maxMatchLen)

	err := r.lzDecoder.Init(
		lz.WithWindowSize(winSize),
		lz.WithBufferSize(bufSize),
	)
	if err != nil {
		return err
	}

	r.state.init(hdr.Properties)
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
		if a := r.lzDecoder.Available(); a < maxMatchLen {
			return nil
		}

		seq, err := r.readSeq()
		if err != nil {
			s := r.size
			switch err {
			case errEOS:
				if r.rd.possiblyAtEnd() && (s < 0 || s == r.lzDecoder.Off) {
					err = io.EOF
				}
			case io.EOF:
				if !r.rd.possiblyAtEnd() || s != r.lzDecoder.Off {
					err = io.ErrUnexpectedEOF
				}
			}
			return err
		}
		if seq.MatchLen == 0 {
			if err = r.lzDecoder.WriteByte(byte(seq.Aux)); err != nil {
				panic(err)
			}
		} else {
			_, err = r.lzDecoder.WriteMatch(seq.MatchLen,
				seq.Offset)
			if err != nil {
				return err
			}
		}
		if r.size == r.lzDecoder.Off {
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
}

// Read reads data from the dictionary and refills it if needed.
func (r *Reader) Read(p []byte) (n int, err error) {
	for {
		// Read from a dictionary never returns an error
		k, _ := r.lzDecoder.Read(p[n:])
		n += k
		if n == len(p) {
			return n, nil
		}
		if r.err != nil {
			return n, r.err
		}
		if err = r.fillBuffer(); err != nil {
			r.err = err
		}
	}
}
