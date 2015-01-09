package lzma

import (
	"bufio"
	"io"

	"github.com/uli-go/xz/xlog"
)

// defaultBufferLen defines the default buffer length
const defaultBufferLen = 4096

// NoUnpackLen provides the header value for an EOS marker in the stream.
const noUnpackLen int64 = -1

// Reader is able to read a LZMA byte stream and to read the plain text.
type Reader struct {
	dict      *readerDict
	or        *opReader
	unpackLen int64
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
	if err = verifyProperties(properties); err != nil {
		return nil, err
	}
	u, err := readUint64LE(f)
	if err != nil {
		return nil, err
	}
	unpackLen := int64(u)
	if unpackLen < noUnpackLen {
		newError("unpack length greater than MaxInt64 not supported")
	}

	historyLen := int(properties.DictLen)
	l := &Reader{unpackLen: unpackLen}
	l.dict, err = newReaderDict(historyLen, defaultBufferLen)
	if err != nil {
		return nil, err
	}
	l.or, err = newOpReader(f, properties, l.dict)
	if err != nil {
		return nil, err
	}
	l.or.properties.Len = unpackLen
	if unpackLen == noUnpackLen {
		l.or.properties.EOS = true
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
	for l.dict.Readable() < l.dict.bufferLen {
		op, err := l.or.ReadOp()
		if err != nil {
			switch {
			case err == eos:
				if l.unpackLen != noUnpackLen &&
					l.dict.Offset() != l.unpackLen {
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
		xlog.Printf(debug, "op %s", op)

		if err = op.applyReaderDict(l.dict); err != nil {
			return err
		}
		if l.unpackLen != noUnpackLen && l.dict.Offset() > l.unpackLen {
			return newError("actual uncompressed length too large")
		}
		if l.dict.Offset() == l.unpackLen {
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
