package lzma

import (
	"bufio"
	"io"

	"github.com/uli-go/xz/xlog"
)

// defaultBufferLen defines the default buffer length
const defaultBufferLen = 4096

// noHeaderLen defines the value of the length field in the LZMA header.
const noHeaderLen uint64 = 1<<64 - 1

// Reader supports the reading of LZMA byte streams.
type Reader struct {
	dict  *readerDict
	or    *opReader
	props *Properties
}

// readHeader reads the classic header for LZMA files.
func readHeader(r io.Reader) (p *Properties, err error) {
	p, err = readProperties(r)
	if err != nil {
		return nil, err
	}
	u, err := readUint64LE(r)
	if err != nil {
		return nil, err
	}
	if u == noHeaderLen {
		p.Len = 0
		p.EOS = true
		p.LenInHeader = false
		return p, nil
	}
	p.Len = int64(u)
	if p.Len < 0 {
		return nil, newError(
			"unpack length in header not supported by int64")
	}
	p.EOS = false
	p.LenInHeader = true
	return p, nil
}

// NewReader creates a reader for LZMA byte streams. It reads the LZMA file
// header.
func NewReader(r io.Reader) (*Reader, error) {
	f := bufio.NewReader(r)
	p, err := readHeader(f)
	if err != nil {
		return nil, err
	}
	if err = verifyProperties(p); err != nil {
		return nil, err
	}
	lr := &Reader{props: p}
	lr.dict, err = newReaderDict(int(p.DictLen), defaultBufferLen)
	if err != nil {
		return nil, err
	}
	lr.or, err = newOpReader(f, p, lr.dict)
	if err != nil {
		return nil, err
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
				if lr.props.LenInHeader &&
					lr.dict.Offset() != lr.props.Len {
					return errUnexpectedEOS
				}
				lr.dict.closed = true
				if !lr.or.rd.possiblyAtEnd() {
					return newError("data after eos")
				}
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
		if lr.props.LenInHeader && lr.dict.Offset() >= lr.props.Len {
			if lr.dict.Offset() > lr.props.Len {
				return newError(
					"more data than announced in header")
			}
			lr.dict.closed = true
			if !lr.or.rd.possiblyAtEnd() {
				if _, err = lr.or.ReadOp(); err != eos {
					return newError(
						"wrong length in header")
				}
				if !lr.or.rd.possiblyAtEnd() {
					return newError("data after eos")
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
	return *lr.props
}
