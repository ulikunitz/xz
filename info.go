package xz

import (
	"errors"
	"io"

	"github.com/ulikunitz/xz/lzma"
)

// skip skips over the next n bytes in the reader.
func skip(r io.Reader, n int64) error {
	if n < 0 {
		panic("n < 0")
	}

	if s, ok := r.(io.Seeker); ok {
		_, err := s.Seek(n, io.SeekCurrent)
		if err == nil {
			return nil
		}
	}

	p := make([]byte, 16*1024)
	for n > 0 {
		if n < int64(len(p)) {
			p = p[:n]
		}
		k, err := r.Read(p)
		n -= int64(k)
		if err != nil {
			return err
		}
	}

	return nil
}

// If the no information about chunks should be provided the blockHeader needs
// to provide the error message.
var errSuppressChunks = errors.New("suppress info on chunks")

// A walker is used to visit all important parts of the XZ stream. It cannot
// provide the data itself.
type walker interface {
	streamHeader(h header) error
	streamFooter(f footer) error
	blockHeader(bh blockHeader) error
	chunkHeader(ch lzma.ChunkHeader) error
	index(records int) error
	record(r record) error
}

const (
	singleStream = 1 << iota
)

func walk(r io.Reader, w walker, flags byte) error {

	return errors.ErrUnsupported
}
