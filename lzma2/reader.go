// Package lzma2 provides a reader and a writer for the LZMA2 encoding.
// LZMA2 is a framing format for LZMA raw streams to support flushing,
// parallel compression and uncompressed segments.
//
// The Reader and Writer allows the reading and writing of LZMA2 chunk
// sequences. They can be used to parallel compress or decompress LZMA2
// streams.
package lzma2

import (
	"errors"
	"io"

	"github.com/ulikunitz/xz/lzma"
)

// ReaderParams defines the LZMA2 reader parameters.
type ReaderParams struct {
	DictCap int
}

// ReaderDefaults define the defaults for the reader parameters.
var ReaderDefaults = ReaderParams{
	DictCap: 8 * 1024 * 1024,
}

// Verify verifies the LZMA2 reader parameters for correctness.
func (p *ReaderParams) Verify() error {
	return verifyDictCap(p.DictCap)
}

// Reader supports the reading of LZMA2 chunk sequences. Note that the
// first chunk should have a dictionary reset and the first compressed
// chunk a properties reset. The chunk sequence may not be terminated by
// an end-of-stream chunk.
type Reader struct {
	r   io.Reader
	err error

	dict        *lzma.DecoderDict
	ur          *uncompressedReader
	decoder     *lzma.Decoder
	chunkReader io.Reader

	cstate chunkState
	ctype  chunkType
}

// NewReader creates a reader for an LZMA2 chunk sequence with the given
// dictionary capacity.
func NewReader(lzma2 io.Reader, dictCap int) (r *Reader, err error) {
	params := ReaderDefaults
	params.DictCap = dictCap
	return NewReaderParams(lzma2, &params)
}

// NewReaderParams creates a new LZMA2 reader using the given
// parameters.
func NewReaderParams(lzma2 io.Reader, params *ReaderParams) (r *Reader, err error) {
	if err = params.Verify(); err != nil {
		return nil, err
	}
	r = &Reader{
		r:      lzma2,
		cstate: start,
	}
	r.dict, err = lzma.NewDecoderDict(params.DictCap)
	if err != nil {
		return nil, err
	}
	if err = r.startChunk(); err != nil {
		return nil, err
	}
	return r, nil
}

// uncompressed tests whether the chunk type specifies an uncompressed
// chunk.
func uncompressed(ctype chunkType) bool {
	return ctype == cU || ctype == cUD
}

// startChunk parses a new chunk.
func (r *Reader) startChunk() error {
	r.chunkReader = nil
	header, err := readChunkHeader(r.r)
	if err != nil {
		if err == io.EOF {
			err = io.ErrUnexpectedEOF
		}
		return err
	}
	if err = r.cstate.next(header.ctype); err != nil {
		return err
	}
	if r.cstate == stop {
		return io.EOF
	}
	if header.ctype == cUD || header.ctype == cLRND {
		r.dict.Reset()
	}
	size := int64(header.uncompressed) + 1
	if uncompressed(header.ctype) {
		if r.ur != nil {
			r.ur.Reopen(r.r, size)
		} else {
			r.ur = newUncompressedReader(r.r, r.dict, size)
		}
		r.chunkReader = r.ur
		return nil
	}
	br := lzma.ByteReader(io.LimitReader(r.r, int64(header.compressed)+1))
	if r.decoder == nil {
		state := lzma.NewState(header.props)
		r.decoder, err = lzma.NewDecoder(br, state, r.dict, size)
		if err != nil {
			return err
		}
		r.chunkReader = r.decoder
		return nil
	}
	switch header.ctype {
	case cLR:
		r.decoder.State.Reset()
	case cLRN, cLRND:
		r.decoder.State = lzma.NewState(header.props)
	}
	err = r.decoder.Reopen(br, size)
	if err != nil {
		return err
	}
	r.chunkReader = r.decoder
	return nil
}

// Read reads data from the LZMA2 chunk sequence.
func (r *Reader) Read(p []byte) (n int, err error) {
	if r.err != nil {
		return 0, r.err
	}
	for n < len(p) {
		var k int
		k, err = r.chunkReader.Read(p[n:])
		n += k
		if err == io.EOF {
			err = r.startChunk()
		}
		if err != nil {
			r.err = err
			return n, err
		}
		if k == 0 {
			r.err = errors.New("lzma2: Reader doesn't get data")
			return n, r.err
		}
	}
	return n, nil
}

// EOS returns whether the LZMA2 stream has been terminated by an
// end-of-stream chunk.
func (r *Reader) EOS() bool {
	return r.cstate == stop
}

// uncompressedReader is used to read uncompressed chunks.
type uncompressedReader struct {
	lr   io.LimitedReader
	Dict *lzma.DecoderDict
	eof  bool
	err  error
}

// newUncompressedReader initializes a new uncompressedReader.
func newUncompressedReader(r io.Reader, dict *lzma.DecoderDict, size int64) *uncompressedReader {
	ur := &uncompressedReader{
		lr:   io.LimitedReader{R: r, N: size},
		Dict: dict,
	}
	return ur
}

// Reopen reinitializes an uncompressed reader.
func (ur *uncompressedReader) Reopen(r io.Reader, size int64) {
	ur.eof = false
	ur.lr = io.LimitedReader{R: r, N: size}
}

// fill reads uncompressed data into the dictionary.
func (ur *uncompressedReader) fill() error {
	if !ur.eof {
		n, err := io.CopyN(ur.Dict, &ur.lr, int64(ur.Dict.Available()))
		if err != io.EOF {
			return err
		}
		ur.eof = true
		if n > 0 {
			return nil
		}
	}
	if ur.lr.N != 0 {
		return io.ErrUnexpectedEOF
	}
	return io.EOF
}

// Read reads uncompressed data from the limited reader.
func (ur *uncompressedReader) Read(p []byte) (n int, err error) {
	if ur.err != nil {
		return 0, ur.err
	}
	for {
		var k int
		k, err = ur.Dict.Read(p[n:])
		n += k
		if n >= len(p) {
			return n, nil
		}
		if err != nil {
			break
		}
		err = ur.fill()
		if err != nil {
			break
		}
	}
	ur.err = err
	return n, err
}
