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

// errSuppressRecords signals that the user is not interested in all the
// records.
var errSuppressRecords = errors.New("suppress info on records")

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
	w        walker
	dr       discard.Reader
	h        header
	checkLen int
}

func newStreamWalkParser(r io.Reader, w walker, h header) *streamWalkParser {
	var checkLen int
	f := h.flags & 0x0f
	if f == 0 {
		checkLen = 0
	} else {
		// this works because the length are grouped in triples
		checkLen = 1 << (((f - 1) / 3) + 2)
	}
	return &streamWalkParser{
		w:        w,
		dr:       discard.Wrap(r),
		h:        h,
		checkLen: checkLen,
	}
}

func (p *streamWalkParser) Block() error {
	hdr, _, err := readBlockHeader(p.dr)
	if err != nil {
		return err
	}
	requireChunks := true
	err = p.w.blockHeader(*hdr)
	if err != nil {
		if err == errSuppressChunks {
			requireChunks = false
		} else {
			return err
		}
	}
	if requireChunks || hdr.compressedSize < 0 {
		err = lzma.Walk2(p.dr, func(ch lzma.ChunkHeader) error {
			return p.w.chunkHeader(ch)
		})
		if err != nil {
			if err == io.EOF {
				err = io.ErrUnexpectedEOF
			}
			return err
		}
	} else {
		if _, err = p.dr.Discard64(hdr.compressedSize); err != nil {
			if err == io.EOF {
				err = io.ErrUnexpectedEOF
			}
			return err
		}
	}
	k := padLen(hdr.compressedSize) + p.checkLen
	if _, err := p.dr.Discard64(int64(k)); err != nil {
		if err == io.EOF {
			err = io.ErrUnexpectedEOF
		}
		return err
	}
	return nil
}

func (p *streamWalkParser) Body() error {
	var err error
	if err = p.w.streamHeader(p.h); err != nil {
		return err
	}

	// parse blocks
	for {
		if err = p.Block(); err != nil {
			if err == errIndexIndicator {
				break
			}
			return err
		}
	}

	// Index
	br := byteReader(p.dr)
	indexLen := 1
	u, n, err := readUvarint(br)
	indexLen += n
	if err != nil {
		if err == io.EOF {
			err = io.ErrUnexpectedEOF
		}
		return err
	}
	recLen := int(u)
	if recLen < 0 || uint64(recLen) != u {
		return errors.New("xz: record number overflow")
	}
	notifyRecords := true
	if err = p.w.index(n); err != nil {
		if err == errSuppressRecords {
			notifyRecords = false
		} else {
			return err
		}
	}
	for range n {
		record, n, err := readRecord(br)
		indexLen += n
		if err != nil {
			if err == io.EOF {
				err = io.ErrUnexpectedEOF
			}
			return err
		}
		if notifyRecords {
			if err = p.w.record(record); err != nil {
				return err
			}
		}
	}
	k := padLen(int64(indexLen)) + 4
	if _, err = p.dr.Discard64(int64(k)); err != nil {
		if err == io.EOF {
			err = io.ErrUnexpectedEOF
		}
		return err
	}

	// footer
	buf := make([]byte, footerLen)
	if _, err = io.ReadFull(p.dr, buf); err != nil {
		if err == io.EOF {
			err = io.ErrUnexpectedEOF
		}
		return err
	}
	var f footer
	if err = f.UnmarshalBinary(buf); err != nil {
		return err
	}
	return p.w.streamFooter(f)
}

func walkStream(r io.Reader, w walker, flags byte) error {
	h, err := readHeader(r, flags)
	if err != nil {
		return err
	}
	p := newStreamWalkParser(r, w, h)
	return p.Body()
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

// TODO: implement the actual Stat function
