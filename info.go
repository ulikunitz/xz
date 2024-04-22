package xz

import (
	"errors"
	"io"

	"github.com/ulikunitz/xz/internal/discard"
	"github.com/ulikunitz/xz/lzma"
)

// errSuppressChunks should be returned from the blockHeader callback in the
// [walker] interface to suppress the chunkHeader callback.
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

type streamWalkParser struct {
	w  walker
	dr discard.Reader
	h  header
}

func newStreamWalkParser(r io.Reader, w walker, h header) *streamWalkParser {
	return &streamWalkParser{
		w:  w,
		dr: discard.Wrap(r),
		h:  h,
	}
}

func (p *streamWalkParser) Tail() error {
	if err := p.w.streamHeader(p.h); err != nil {
		return err
	}
	for {
		// find blocks
		panic("TODO")
	}
}


func walkStream(r io.Reader, w walker, flags byte) error {
	h, err := readHeader(r, flags)
	if err != nil {
		return err
	}
	p := newStreamWalkParser(r, w, h)
	return p.Tail()
}

// Flags for the walk function.
const (
	singleStream = 1 << iota
)

func walk(r io.Reader, w walker, flags byte) error {
	var err error
	if flags&singleStream != 0 {
		if err = walkStream(r, w, 0); err != nil {
			return err
		}
		// check for EOF
		var a [1]byte
		if _, err = r.Read(a[:]); err != io.EOF {
			return errors.New(
				"xz: expected EOF at end of single stream")
		}
		return nil
	}

	if err = walkStream(r, w, 0); err != nil {
		return err
	}
	for {
		if err = walkStream(r, w, expectPadding); err != nil {
			if err == io.EOF {
				return nil
			}
			return err
		}
	}
}
