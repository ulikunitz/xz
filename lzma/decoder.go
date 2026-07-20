// Copyright 2014-2022 Ulrich Kunitz. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package lzma

import (
	"errors"
	"fmt"
	"io"
)

// decoder decodes a raw LZMA stream without any header.
type decoder struct {
	// dictionary; the rear pointer of the buffer will be used for
	// reading the data.
	Dict *decoderDict
	// decoder state
	State *state
	// range decoder
	rd *rangeDecoder
	// start stores the head value of the dictionary for the LZMA
	// stream
	start int64
	// size of uncompressed data
	size int64
	// end-of-stream encountered
	eos bool
	// EOS marker found
	eosMarker bool
}

// newDecoder creates a new decoder instance. The parameter size provides
// the expected byte size of the decompressed data. If the size is
// unknown use a negative value. In that case the decoder will look for
// a terminating end-of-stream marker.
func newDecoder(r io.Reader, state *state, dict *decoderDict, size int64) (d *decoder, err error) {
	rd, err := newRangeDecoder(r)
	if err != nil {
		return nil, err
	}
	d = &decoder{
		State: state,
		Dict:  dict,
		rd:    rd,
		size:  size,
		start: dict.pos(),
	}
	return d, nil
}

// Reopen restarts the decoder with a new byte reader and a new size. Reopen
// resets the Decompressed counter to zero.
func (d *decoder) Reopen(r io.Reader, size int64) error {
	var err error
	if d.rd, err = newRangeDecoder(r); err != nil {
		return err
	}
	d.start = d.Dict.pos()
	d.size = size
	d.eos = false
	return nil
}

// decodeLiteral decodes a single literal from the LZMA stream and applies it directly.
func (d *decoder) decodeLiteral() error {
	litState := d.State.litState(d.Dict.byteAt(1), d.Dict.head)
	match := d.Dict.byteAt(int(d.State.rep[0]) + 1)
	s, err := d.State.litCodec.Decode(d.rd, d.State.state, match, litState)
	if err != nil {
		return err
	}
	return d.Dict.WriteByte(s)
}

// errEOS indicates that an EOS marker has been found.
var errEOS = errors.New("EOS marker found")

// processNextOp decodes the next operation from the compressed stream and applies it directly.
func (d *decoder) processNextOp() error {
	const eosDist = 1<<32 - 1

	state, state2, posState := d.State.states(d.Dict.head)

	b, err := d.State.isMatch[state2].Decode(d.rd)
	if err != nil {
		return err
	}
	if b == 0 {
		err := d.decodeLiteral()
		if err != nil {
			return err
		}
		d.State.updateStateLiteral()
		return nil
	}
	b, err = d.State.isRep[state].Decode(d.rd)
	if err != nil {
		return err
	}
	if b == 0 {
		d.State.rep[3], d.State.rep[2], d.State.rep[1] =
			d.State.rep[2], d.State.rep[1], d.State.rep[0]

		d.State.updateStateMatch()
		n, err := d.State.lenCodec.Decode(d.rd, posState)
		if err != nil {
			return err
		}
		d.State.rep[0], err = d.State.distCodec.Decode(d.rd, n)
		if err != nil {
			return err
		}
		if d.State.rep[0] == eosDist {
			d.eosMarker = true
			return errEOS
		}
		return d.Dict.writeMatch(int64(d.State.rep[0])+minDistance, int(n)+minMatchLen)
	}
	b, err = d.State.isRepG0[state].Decode(d.rd)
	if err != nil {
		return err
	}
	dist := d.State.rep[0]
	if b == 0 {
		b, err = d.State.isRepG0Long[state2].Decode(d.rd)
		if err != nil {
			return err
		}
		if b == 0 {
			d.State.updateStateShortRep()
			return d.Dict.writeMatch(int64(dist)+minDistance, 1)
		}
	} else {
		b, err = d.State.isRepG1[state].Decode(d.rd)
		if err != nil {
			return err
		}
		if b == 0 {
			dist = d.State.rep[1]
		} else {
			b, err = d.State.isRepG2[state].Decode(d.rd)
			if err != nil {
				return err
			}
			if b == 0 {
				dist = d.State.rep[2]
			} else {
				dist = d.State.rep[3]
				d.State.rep[3] = d.State.rep[2]
			}
			d.State.rep[2] = d.State.rep[1]
		}
		d.State.rep[1] = d.State.rep[0]
		d.State.rep[0] = dist
	}
	n, err := d.State.repLenCodec.Decode(d.rd, posState)
	if err != nil {
		return err
	}
	d.State.updateStateRep()
	return d.Dict.writeMatch(int64(dist)+minDistance, int(n)+minMatchLen)
}

// decompress fills the dictionary unless no space for new data is
// available. If the end of the LZMA stream has been reached io.EOF will
// be returned.
func (d *decoder) decompress() error {
	if d.eos {
		return io.EOF
	}
	for d.Dict.Available() >= maxMatchLen {
		err := d.processNextOp()
		switch err {
		case nil:
			// break
		case errEOS:
			d.eos = true
			if !d.rd.possiblyAtEnd() {
				return errDataAfterEOS
			}
			if d.size >= 0 && d.size != d.Decompressed() {
				return errSize
			}
			return io.EOF
		case io.EOF:
			d.eos = true
			return io.ErrUnexpectedEOF
		default:
			return err
		}

		if d.size >= 0 && d.Decompressed() >= d.size {
			d.eos = true
			if d.Decompressed() > d.size {
				return errSize
			}
			if !d.rd.possiblyAtEnd() {
				switch err = d.processNextOp(); err {
				case nil:
					return errSize
				case io.EOF:
					return io.ErrUnexpectedEOF
				case errEOS:
					break
				default:
					return err
				}
			}
			return io.EOF
		}
	}
	return nil
}

// Errors that may be returned while decoding data.
var (
	errDataAfterEOS = errors.New("lzma: data after end of stream marker")
	errSize         = errors.New("lzma: wrong uncompressed data size")
)

// Read reads data from the buffer. If no more data is available io.EOF is
// returned.
func (d *decoder) Read(p []byte) (n int, err error) {
	var k int
	for {
		// Read of decoder dict never returns an error.
		k, err = d.Dict.Read(p[n:])
		if err != nil {
			panic(fmt.Errorf("dictionary read error %s", err))
		}
		if k == 0 && d.eos {
			return n, io.EOF
		}
		n += k
		if n >= len(p) {
			return n, nil
		}
		if err = d.decompress(); err != nil && err != io.EOF {
			return n, err
		}
	}
}

// Decompressed returns the number of bytes decompressed by the decoder.
func (d *decoder) Decompressed() int64 {
	return d.Dict.pos() - d.start
}
