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

// Reader supports the reading of LZMA byte streams.
type Reader struct {
	dict      *readerDict
	or        *opReader
	unpackLen int64
}

// NewReader creates a reader for LZMA byte streams.
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
	lr := &Reader{unpackLen: unpackLen}
	lr.dict, err = newReaderDict(historyLen, defaultBufferLen)
	if err != nil {
		return nil, err
	}
	lr.or, err = newOpReader(f, properties, lr.dict)
	if err != nil {
		return nil, err
	}
	lr.or.properties.Len = unpackLen
	if unpackLen == noUnpackLen {
		lr.or.properties.EOS = true
	}
	return lr, nil
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
func (lr *Reader) Read(p []byte) (n int, err error) {
	for {
		var k int
		k, err = lr.dict.Read(p[n:])
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
		if err = lr.fill(); err != nil {
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
func (lr *Reader) fill() error {
	if lr.dict.closed {
		return nil
	}
	for lr.dict.Readable() < lr.dict.bufferLen {
		op, err := lr.or.ReadOp()
		if err != nil {
			switch {
			case err == eos:
				if lr.unpackLen != noUnpackLen &&
					lr.dict.Offset() != lr.unpackLen {
					return errUnexpectedEOS
				}
				lr.dict.closed = true
				return nil
			case err == io.EOF:
				return newError(
					"unexpected end of compressed stream")
			default:
				return err
			}
		}
		xlog.Printf(debug, "op %s", op)

		if err = op.applyReaderDict(lr.dict); err != nil {
			return err
		}
		if lr.unpackLen != noUnpackLen && lr.dict.Offset() > lr.unpackLen {
			return newError("actual uncompressed length too large")
		}
		if lr.dict.Offset() == lr.unpackLen {
			lr.dict.closed = true
			if !lr.or.rd.possiblyAtEnd() {
				if _, err = lr.or.ReadOp(); err != eos {
					return newError(
						"wrong length in header")
				}
			}
			return nil
		}
	}
	return nil
}

// Properties returns the properties of the LZMA reader. The properties reflect
// the status provided by the header of the LZMA file.
func (lr *Reader) Properties() Properties {
	return lr.or.properties
}
