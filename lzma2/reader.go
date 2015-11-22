package lzma2

import (
	"errors"
	"io"

	"github.com/ulikunitz/xz/lzma"
)

// errUnexpectedEOF indicates an unexpected end of file.
var errUnexpectedEOF = errors.New("lzma2: unexpected eof")

// breader converts a reader into a byte reader.
type breader struct {
	io.Reader
}

// ReadByte read byte function.
func (r breader) ReadByte() (c byte, err error) {
	var p [1]byte
	n, err := r.Reader.Read(p[:])
	if n < 1 {
		if err == nil {
			err = errors.New("ReadByte: no data")
		}
		return 0, err
	}
	return p[0], nil
}

// A reader supports the reading of LZMA2 chunk sequences. Note that the
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
	header      *chunkHeader

	cstate chunkState
	ctype  chunkType
}

// NewReader creates a reader for an LZMA2 chunk sequence with the given
// dictionary capacity.
func NewReader(lzma2 io.Reader, dictCap int) (r *Reader, err error) {
	panic("TODO")
}

func uncompressed(ctype chunkType) bool {
	return ctype == cU || ctype == cUD
}

func (r *Reader) startChunk() error {
	var err error
	r.chunkReader = nil
	if r.header, err = readChunkHeader(r.r); r.err != nil {
		if err == io.EOF {
			err = errUnexpectedEOF
		}
		return err
	}
	if err = r.cstate.next(r.header.ctype); err != nil {
		return err
	}
	if r.cstate == stop {
		return io.EOF
	}
	if r.header.ctype == cUD || r.header.ctype == cLRND {
		r.dict.Reset()
	}
	size := int64(r.header.uncompressed)
	if uncompressed(r.header.ctype) {
		if r.ur != nil {
			r.ur.Reopen(r.r, size)
		} else {
			r.ur = newUncompressedReader(r.r, r.dict, size)
		}
		r.chunkReader = r.ur
		return nil
	}
	br := breader{io.LimitReader(r.r, int64(r.header.compressed))}
	if r.decoder == nil {
		state := lzma.NewState(r.header.props)
		r.decoder, err = lzma.NewDecoder(br, state, r.dict, size)
		if err != nil {
			return err
		}
		r.chunkReader = r.decoder
		return nil
	}
	switch r.header.ctype {
	case cLR:
		r.decoder.State.Reset()
	case cLRN, cLRND:
		r.decoder.State = lzma.NewState(r.header.props)
	}
	err = r.decoder.Reopen(br, size)
	if err != nil {
		return err
	}
	r.chunkReader = r.decoder
	return nil
}

func NewReaderSize(lzma2 io.Reader, dictCap int, bufSize int) (r *Reader, err error) {

	if lzma2 == nil {
		return nil, errors.New("lzma2: reader must be non-nil")
	}

	r = &Reader{
		r:      lzma2,
		cstate: start,
	}
	if r.dict, err = lzma.NewDecoderDict(dictCap, bufSize); err != nil {
		return nil, err
	}
	if err = r.startChunk(); err != nil {
		if err == io.EOF {
			r.err = err
			return r, nil
		}
		return nil, err
	}
	return r, nil
}

// Read reads data from the LZMA2 chunk sequence. If an end-of-stream
// chunk is encountered EOS is returned, it the sequence stops without
// an end-of-stream chunk io.EOF is returned.
func (r *Reader) Read(p []byte) (n int, err error) {
	if r.err != nil {
		return 0, err
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
			return n, errors.New("lzma2: Reader doesn't get data")
		}
	}
	return n, nil
}

//  EOS returns whether the LZMA2 stream has been terminated by an
//  end-of-stream chunk.
func (r *Reader) EOS() bool {
	return r.cstate == stop
}

// The file reader supports the reading of LZMA2 files, where the first
// chunk is preceded by the dictionary size.
type FileReader struct {
	r Reader
}

// NewFileReader creates a reader for LZMA2 files, where the dictionary
// capacity is encoded in the first byte.
func NewFileReader(lzma2File io.Reader) (r *FileReader, err error) {
	panic("TODO")
}

// Reads data from the file reader. It returns io.EOF if the end of the
// file is encountered.
func (r *FileReader) Read(p []byte) (n int, err error) {
	panic("TODO")
}

type uncompressedReader struct {
	lr   io.LimitedReader
	Dict *lzma.DecoderDict
	eof  bool
	err  error
}

func newUncompressedReader(r io.Reader, dict *lzma.DecoderDict, size int64) *uncompressedReader {
	ur := &uncompressedReader{
		lr:   io.LimitedReader{R: r, N: size},
		Dict: dict,
	}
	return ur
}

func (ur *uncompressedReader) Reopen(r io.Reader, size int64) {
	ur.eof = false
	ur.lr = io.LimitedReader{R: r, N: size}
}

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
		return errUnexpectedEOF
	}
	return io.EOF
}

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
