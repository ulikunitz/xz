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
	CEOS  = byte(0)
	CUD   = byte(0b01)
	CU    = byte(0b10)
	CC    = byte(0b100) << 5
	CCS   = byte(0b101) << 5
	CCSP  = byte(0b110) << 5
	CCSPD = byte(0b111) << 5
	cMask = CCSPD
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
		case CEOS:
			return sF
		case CU:
			switch s {
			case s1:
				return s1
			case s2:
				return s2
			}
		case CUD:
			return s1
		}
	} else {
		switch c & cMask {
		case CC, CCS:
			if s == s2 {
				return s2
			}
		case CCSP:
			if s == s1 || s == s2 {
				return s2
			}
		case CCSPD:
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

// ChunkHeader represents a chunk header.
type ChunkHeader struct {
	Control        byte
	CompressedSize int
	Size           int
	Properties     Properties
}

// peekChunkHeader gets the next chunk header from the buffered reader without
// advancing it.
func peekChunkHeader(r *hdrReader) (h ChunkHeader, n int, err error) {
	p := make([]byte, 1, 6)
	k, err := r.Peek(p)
	if err != nil {
		if k > 0 {
			panic("unexpected")
		}
		return h, 0, err
	}
	n += k
	h.Control = p[0]
	if h.Control&(1<<7) == 0 {
		switch h.Control {
		case CEOS:
			return h, n, nil
		case CU, CUD:
			break
		default:
			return h, n, fmt.Errorf(
				"lzma: unsupported chunk header"+
					" control byte %02x", h.Control)
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
		h.Size = int(getBE16(p[1:3])) + 1
	} else {
		h.Control &= cMask
		switch h.Control {
		case CC, CCS:
			p = p[0:5]
		case CCSP, CCSPD:
			p = p[0:6]
		default:
			return h, n, fmt.Errorf("lzma: unsupported chunk header"+
				" control byte %02x", h.Control)
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
		h.Size = int(p[0]&(1<<5-1))<<16 + int(getBE16(p[1:3])) + 1
		h.CompressedSize = int(getBE16(p[3:5])) + 1
		if h.Control == CCSP || h.Control == CCSPD {
			if err = h.Properties.fromByte(p[5]); err != nil {
				return h, n, err
			}
		}
	}
	return h, n, nil
}

// parseChunkHeader reads the next chunk header from the reader.
func parseChunkHeader(r io.Reader) (h ChunkHeader, err error) {
	p := make([]byte, 1, 6)
	if _, err = io.ReadFull(r, p); err != nil {
		return h, err
	}
	h.Control = p[0]
	if h.Control&(1<<7) == 0 {
		switch h.Control {
		case CEOS:
			// return h, io.EOF
			return h, nil
		case CU, CUD:
			break
		default:
			return h, fmt.Errorf(
				"lzma: unsupported chunk header"+
					" control byte %02x", h.Control)
		}
		if _, err = io.ReadFull(r, p[1:3]); err != nil {
			if err == io.EOF {
				err = io.ErrUnexpectedEOF
			}
			return h, err
		}
		h.Size = int(getBE16(p[1:3])) + 1
	} else {
		h.Control &= cMask
		switch h.Control {
		case CC, CCS:
			p = p[0:5]
		case CCSP, CCSPD:
			p = p[0:6]
		default:
			return h, fmt.Errorf("lzma: unsupported chunk header"+
				" control byte %02x", h.Control)
		}
		if _, err := io.ReadFull(r, p[1:]); err != nil {
			if err == io.EOF {
				err = io.ErrUnexpectedEOF
			}
			return h, err
		}
		h.Size = int(p[0]&(1<<5-1))<<16 + int(getBE16(p[1:3])) + 1
		h.CompressedSize = int(getBE16(p[3:5])) + 1
		if h.Control == CCSP || h.Control == CCSPD {
			if err = h.Properties.fromByte(p[5]); err != nil {
				return h, err
			}
		}
	}
	return h, nil
}

// append appends the binary representation of the chunk header to p. An error
// is returned if the values in chunk header are inconsistent.
func (h ChunkHeader) append(p []byte) (q []byte, err error) {
	if h.Control == CEOS {
		return append(p, CEOS), nil
	}
	var d [6]byte
	d[0] = h.Control
	if h.Control == CU || h.Control == CUD {
		if !(1 <= h.Size && h.Size <= maxChunkSize) {
			return p, fmt.Errorf(
				"lzma: chunk header size %d out of range"+
					" for uncompressed chunk",
				h.Size)
		}
		putBE16(d[1:], uint16(h.Size-1))
		return append(p, d[:3]...), nil
	}
	if !(1 <= h.Size && h.Size <= maxUncompressedChunkSize) {
		return p, errors.New(
			"lzma: chunk header uncompressed size out of range")
	}
	if !(1 <= h.CompressedSize && h.CompressedSize <= maxChunkSize) {
		return p, fmt.Errorf("lzma: chunk header compressed size %d"+
			" is out of range", h.CompressedSize)
	}
	us := h.Size - 1
	d[0] |= byte(us >> 16)
	putBE16(d[1:], uint16(us))
	putBE16(d[3:], uint16(h.CompressedSize-1))
	if h.Control == CC || h.Control == CCS {
		return append(p, d[:5]...), nil
	}
	d[5] = h.Properties.byte()
	if h.Control == CCSP || h.Control == CCSPD {
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
	r.cstate = r.cstate.next(h.Control)
	if r.cstate == sErr {
		return fmt.Errorf("lzma: unexpected byte control header %02x",
			h.Control)
	}
	if r.cstate == sF {
		return io.EOF
	}

	if h.Control == CUD || h.Control == CCSPD {
		// Not strictly necessary, but ensure that there is no
		// error in the matches that follow.
		r.buffer.Reset()
	}

	if h.Control == CU || h.Control == CUD {
		// copy uncompressed data directly into the dictionary
		_, err = io.CopyN(&r.buffer, r.r, int64(h.Size))
		if err == io.EOF {
			err = io.ErrUnexpectedEOF
		}
		return err
	}

	switch h.Control {
	case CCSP, CCSPD:
		r.state.init(h.Properties)
	case CCS:
		r.state.reset()
	}

	lr := io.LimitReader(r.r, int64(h.CompressedSize))
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
	n := h.Size
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
