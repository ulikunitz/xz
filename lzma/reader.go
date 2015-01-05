package lzma

import (
	"bufio"
	"io"
)

// bufferLen is the value used for the bufferLen used by the decoder.
var bufferLen = 64 * (1 << 10)

// NoUnpackLen provides the header value for an EOS marker in the stream.
const NoUnpackLen uint64 = 1<<64 - 1

// Reader is able to read a LZMA byte stream and to read the plain text.
type Reader struct {
	dict       readerDict
	or         *opReader
	unpackLen  uint64
	currentLen uint64
}

// NewReader creates an LZMA reader. It reads the classic, original LZMA
// format. Note that LZMA2 uses a different header format.
func NewReader(r io.Reader) (*Reader, error) {

	// read header
	f := bufio.NewReader(r)
	properties, err := readProperties(f)
	if err != nil {
		return nil, err
	}
	historyLen := int(properties.DictLen)
	if historyLen < 0 {
		return nil, newError(
			"property DictLen exceeds maximum int value")
	}
	unpackLen, err := readUint64LE(f)
	if err != nil {
		return nil, err
	}

	l := &Reader{unpackLen: unpackLen}
	if err = l.dict.init(historyLen, bufferLen); err != nil {
		return nil, err
	}
	if l.or, err = newOpReader(f, properties, &l.dict); err != nil {
		return nil, err
	}
	return l, nil
}

// getUint64LE converts the uint64 value stored as little endian to an uint64
// value.
func getUint64LE(b []byte) uint64 {
	x := uint64(b[7]) << 56
	x |= uint64(b[6]) << 48
	x |= uint64(b[5]) << 40
	x |= uint64(b[4]) << 32
	x |= uint64(b[3]) << 24
	x |= uint64(b[2]) << 16
	x |= uint64(b[1]) << 8
	x |= uint64(b[0])
	return x
}

// readUint64LE reads a uint64 little-endian integer from reader.
func readUint64LE(r io.Reader) (x uint64, err error) {
	b := make([]byte, 8)
	if _, err = io.ReadFull(r, b); err != nil {
		return 0, err
	}
	x = getUint64LE(b)
	return x, nil
}

// Reads reads data from the decoder stream.
//
// The method might block and is not reentrant.
//
// The end of the LZMA stream is indicated by EOF. There might be other errors
// returned. The decoder will not be able to recover from an error returned.
func (l *Reader) Read(p []byte) (n int, err error) {
	for {
		var k int
		k, err = l.dict.Read(p[n:])
		n += k
		switch {
		case err == io.EOF:
			if n <= 0 {
				return 0, io.EOF
			}
			return n, nil
		case err != nil:
			return n, err
		case n == len(p):
			return n, nil
		}
		if err = l.fill(); err != nil {
			return n, err
		}
	}
}

// Indicates that the end of stream marker has been unexpected.
var errUnexpectedEOS = newError("unexpected end-of-stream marker")

// errWrongTermination indicates that a termination symbol has been received,
// but the range decoder could still produces more data
var errWrongTermination = newError("end of stream marker at wrong place")

// fill puts at lest the requested number of bytes into the decoder dictionary.
func (l *Reader) fill() error {
	if l.dict.closed {
		return nil
	}
	for l.dict.readable() < l.dict.bufferLen {
		op, err := l.or.ReadOp()
		if err != nil {
			switch {
			case err == eos:
				if l.unpackLen != NoUnpackLen &&
					l.currentLen != l.unpackLen {
					return errUnexpectedEOS
				}
				l.dict.closed = true
				return nil
			case err == io.EOF:
				return newError(
					"unexpected end of compressed stream")
			default:
				return err
			}
		}

		n := l.currentLen + uint64(op.Len())
		if n < l.currentLen {
			return newError(
				"negative op length or overflow of decodedLen")
		}
		if n > l.unpackLen {
			l.dict.closed = true
			return newError("decoded stream too long")
		}
		l.currentLen = n

		if err = op.applyReaderDict(&l.dict); err != nil {
			return err
		}
		if n == l.unpackLen {
			l.dict.closed = true
			if !l.or.rd.possiblyAtEnd() {
				if _, err = l.or.ReadOp(); err != eos {
					return newError(
						"wrong length in header")
				}
			}
			return nil
		}
	}
	return nil
}

// Properties returns the properties of the LZMA reader.
func (l *Reader) Properties() Properties {
	return l.or.properties
}
