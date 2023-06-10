// SPDX-FileCopyrightText: © 2014 Ulrich Kunitz
//
// SPDX-License-Identifier: BSD-3-Clause

package lzma

import (
	"bufio"
	"errors"
	"fmt"
	"io"

	"github.com/ulikunitz/lz"
)

// Possible values of the masked control byte in the LZMA2 chunk header. Note
// that the chunk header might contain length bits, so it has to be masked by
// cMask.
const (
	cEOS  = byte(0)
	cUD   = byte(0b01)
	cU    = byte(0b10)
	cC    = byte(0b100) << 5
	cCS   = byte(0b101) << 5
	cCSP  = byte(0b110) << 5
	cCSPD = byte(0b111) << 5
	cMask = cCSPD
)

// chunkState reflects the status of a chunk stream.
type chunkState byte

const (
	sS chunkState = iota
	s1
	s2
	sF
	sErr
)

// chunkState is modified using the given control byte. If an error occurs the
// state becomes sErr.
func (s chunkState) next(c byte) chunkState {
	if s == sF || s == sErr {
		return sErr
	}
	if c&(1<<7) == 0 {
		switch c {
		case cEOS:
			return sF
		case cU:
			switch s {
			case s1:
				return s1
			case s2:
				return s2
			}
		case cUD:
			return s1
		}
	} else {
		switch c & cMask {
		case cC, cCS:
			if s == s2 {
				return s2
			}
		case cCSP:
			if s == s1 || s == s2 {
				return s2
			}
		case cCSPD:
			return s2
		}
	}
	return sErr
}

// chunkReader is used to read a sequence of chunks
type chunkReader struct {
	decoder
	r      io.Reader
	bufr   *bufio.Reader
	cstate chunkState
	err    error
	noEOS  bool
}

// init initializes the chunk reader. Note that the chunk reader at least consumes twice
// the dictSize to support a linear buffer or 2 MiB.
func (r *chunkReader) init(z io.Reader, dictSize int) error {
	*r = chunkReader{r: z}
	dc := lz.DecoderConfig{
		WindowSize: dictSize,
		BufferSize: 2 * dictSize,
	}
	if dc.BufferSize < maxUncompressedChunkSize {
		dc.BufferSize = maxUncompressedChunkSize
	}
	err := r.buffer.Init(dc)
	return err
}

// reset reinitialized the chunkReader. If possible existing allocated data
// should be reused. The function doesn't touch the noEOS flag.
func (r *chunkReader) reset(z io.Reader) {
	r.r = z
	r.buffer.Reset()
	r.cstate = sS
	r.err = nil
}

// chunkHeader represents a chunk header.
type chunkHeader struct {
	control        byte
	compressedSize int
	size           int
	properties     Properties
}

// peekChunkHeader gets the next chunk header from the buffered reader without
// advancing it.
func peekChunkHeader(r *hdrReader) (h chunkHeader, n int, err error) {
	p := make([]byte, 1, 6)
	k, err := r.Peek(p)
	if err != nil {
		if k > 0 {
			panic("unexpected")
		}
		return h, 0, err
	}
	n += k
	h.control = p[0]
	if h.control&(1<<7) == 0 {
		switch h.control {
		case cEOS:
			return h, n, nil
		case cU, cUD:
			break
		default:
			return h, n, fmt.Errorf(
				"lzma: unsupported chunk header"+
					" control byte %02x", h.control)
		}
		n, err = r.Peek(p[:3])
		if err != nil {
			if n == 3 {
				panic("unexpected")
			}
			if err == io.EOF {
				err = io.ErrUnexpectedEOF
			}
			return h, n, err
		}
		h.size = int(getBE16(p[1:3])) + 1
	} else {
		h.control &= cMask
		switch h.control {
		case cC, cCS:
			p = p[0:5]
		case cCSP, cCSPD:
			p = p[0:6]
		default:
			return h, n, fmt.Errorf("lzma: unsupported chunk header"+
				" control byte %02x", h.control)
		}
		n, err = r.Peek(p)
		if err != nil {
			if n == len(p) {
				panic("unexpected")
			}
			if err == io.EOF {
				err = io.ErrUnexpectedEOF
			}
			return h, n, err
		}
		h.size = int(p[0]&(1<<5-1))<<16 + int(getBE16(p[1:3])) + 1
		h.compressedSize = int(getBE16(p[3:5])) + 1
		if h.control == cCSP || h.control == cCSPD {
			if err = h.properties.fromByte(p[5]); err != nil {
				return h, n, err
			}
		}
	}
	return h, n, nil
}

// parseChunkHeader reads the next chunk header from the reader.
func parseChunkHeader(r io.Reader) (h chunkHeader, err error) {
	p := make([]byte, 1, 6)
	if _, err = io.ReadFull(r, p); err != nil {
		return h, err
	}
	h.control = p[0]
	if h.control&(1<<7) == 0 {
		switch h.control {
		case cEOS:
			// return h, io.EOF
			return h, nil
		case cU, cUD:
			break
		default:
			return h, fmt.Errorf(
				"lzma: unsupported chunk header"+
					" control byte %02x", h.control)
		}
		if _, err = io.ReadFull(r, p[1:3]); err != nil {
			if err == io.EOF {
				err = io.ErrUnexpectedEOF
			}
			return h, err
		}
		h.size = int(getBE16(p[1:3])) + 1
	} else {
		h.control &= cMask
		switch h.control {
		case cC, cCS:
			p = p[0:5]
		case cCSP, cCSPD:
			p = p[0:6]
		default:
			return h, fmt.Errorf("lzma: unsupported chunk header"+
				" control byte %02x", h.control)
		}
		if _, err := io.ReadFull(r, p[1:]); err != nil {
			if err == io.EOF {
				err = io.ErrUnexpectedEOF
			}
			return h, err
		}
		h.size = int(p[0]&(1<<5-1))<<16 + int(getBE16(p[1:3])) + 1
		h.compressedSize = int(getBE16(p[3:5])) + 1
		if h.control == cCSP || h.control == cCSPD {
			if err = h.properties.fromByte(p[5]); err != nil {
				return h, err
			}
		}
	}
	return h, nil
}

// append appends the binary representation of the chunk header to p. An error
// is returned if the values in chunk header are inconsistent.
func (h chunkHeader) append(p []byte) (q []byte, err error) {
	if h.control == cEOS {
		return append(p, cEOS), nil
	}
	var d [6]byte
	d[0] = h.control
	if h.control == cU || h.control == cUD {
		if !(1 <= h.size && h.size <= maxChunkSize) {
			return p, fmt.Errorf(
				"lzma: chunk header size %d out of range"+
					" for uncompressed chunk",
				h.size)
		}
		putBE16(d[1:], uint16(h.size-1))
		return append(p, d[:3]...), nil
	}
	if !(1 <= h.size && h.size <= maxUncompressedChunkSize) {
		return p, errors.New(
			"lzma: chunk header uncompressed size out of range")
	}
	if !(1 <= h.compressedSize && h.compressedSize <= maxChunkSize) {
		return p, fmt.Errorf("lzma: chunk header compressed size %d"+
			" is out of range", h.compressedSize)
	}
	us := h.size - 1
	d[0] |= byte(us >> 16)
	putBE16(d[1:], uint16(us))
	putBE16(d[3:], uint16(h.compressedSize-1))
	if h.control == cC || h.control == cCS {
		return append(p, d[:5]...), nil
	}
	d[5] = h.properties.byte()
	if h.control == cCSP || h.control == cCSPD {
		return append(p, d[:6]...), nil

	}
	return p, errors.New("lzma: invalid chunk header")
}

// readChunk reads a single chunk.
func (r *chunkReader) readChunk() error {
	h, err := parseChunkHeader(r.r)
	if err != nil {
		if !r.noEOS && err == io.EOF {
			err = io.ErrUnexpectedEOF
		}
		return err
	}
	r.cstate = r.cstate.next(h.control)
	if r.cstate == sErr {
		return fmt.Errorf("lzma: unexpected byte control header %02x",
			h.control)
	}
	if r.cstate == sF {
		return io.EOF
	}

	if h.control == cUD || h.control == cCSPD {
		// Not strictly necessary, but ensure that there is no
		// error in the matches that follow.
		r.buffer.Reset()
	}

	if h.control == cU || h.control == cUD {
		// copy uncompressed data directly into the dictionary
		_, err = io.CopyN(&r.buffer, r.r, int64(h.size))
		if err == io.EOF {
			err = io.ErrUnexpectedEOF
		}
		return err
	}

	switch h.control {
	case cCSP, cCSPD:
		r.state.init(h.properties)
	case cCS:
		r.state.reset()
	}

	lr := io.LimitReader(r.r, int64(h.compressedSize))
	if r.bufr == nil {
		r.bufr = bufio.NewReader(lr)
	} else {
		r.bufr.Reset(lr)
	}
	if err = r.decoder.rd.init(r.bufr); err != nil {
		if err == io.EOF {
			err = io.ErrUnexpectedEOF
		}
		return err
	}
	n := h.size
	for n > 0 {
		seq, err := r.decoder.readSeq()
		if err != nil {
			if err == io.EOF {
				err = io.ErrUnexpectedEOF
			}
			return err
		}
		if seq.MatchLen == 0 {
			if err = r.buffer.WriteByte(byte(seq.Aux)); err != nil {
				panic(err)
			}
			n--
			continue
		}

		n -= int(seq.MatchLen)
		_, err = r.buffer.WriteMatch(seq.MatchLen, seq.Offset)
		if err != nil {
			return err
		}
	}

	if n < 0 || !r.rd.possiblyAtEnd() {
		return ErrEncoding
	}

	return nil
}

// Read reads data from the chunk reader.
func (r *chunkReader) Read(p []byte) (n int, err error) {
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
		if err = r.readChunk(); err != nil {
			r.err = err
			k := len(r.buffer.Data) - r.buffer.R
			if k > 0 {
				continue
			}
			return n, err
		}
	}
}

// WriteTo supports the WriterTo interface.
func (r *chunkReader) WriteTo(w io.Writer) (n int64, err error) {
	k := len(r.buffer.Data) - r.buffer.R
	if r.err != nil && k == 0 {
		return 0, r.err
	}
	for {
		k, err := r.buffer.WriteTo(w)
		n += k
		if err != nil {
			r.err = err
			return n, err
		}
		if r.err != nil {
			if r.err == io.EOF {
				return n, nil
			}
			return n, r.err
		}
		if err = r.readChunk(); err != nil {
			r.err = err
			k := len(r.buffer.Data) - r.buffer.R
			if k > 0 {
				continue
			}
			if err == io.EOF {
				err = nil
			}
			return n, err
		}
	}
}
