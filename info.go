package xz

import (
	"errors"
	"io"

	"github.com/ulikunitz/xz/internal/stream"
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
	s        stream.Streamer
	h        header
	checkLen int
}

func newStreamWalkParser(s stream.Streamer, w walker, h header) *streamWalkParser {
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
		s:        s,
		h:        h,
		checkLen: checkLen,
	}
}

func (p *streamWalkParser) Block() error {
	hdr, _, err := readBlockHeader(p.s)
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
		hdr.compressedSize, err = lzma.Walk2(p.s, func(ch lzma.ChunkHeader) error {
			return p.w.chunkHeader(ch)
		})
		if err != nil {
			if err == io.EOF {
				err = io.ErrUnexpectedEOF
			}
			return err
		}
	} else {
		if _, err = p.s.Discard64(hdr.compressedSize); err != nil {
			if err == io.EOF {
				err = io.ErrUnexpectedEOF
			}
			return err
		}
	}

	k := padLen(hdr.compressedSize) + p.checkLen
	if _, err := p.s.Discard64(int64(k)); err != nil {
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
	br := byteReader(p.s)
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
	if _, err = p.s.Discard64(int64(k)); err != nil {
		if err == io.EOF {
			err = io.ErrUnexpectedEOF
		}
		return err
	}

	// footer
	buf := make([]byte, footerLen)
	if _, err = io.ReadFull(p.s, buf); err != nil {
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

func walkStream(s stream.Streamer, w walker, flags byte) error {
	h, err := readHeader(s, flags)
	if err != nil {
		return err
	}
	p := newStreamWalkParser(s, w, h)
	return p.Body()
}

// Flags for th  Stat function
const (
	SingleStream = 1 << iota
)

// walk visits all important parts of the XZ stream and calls the methods of
// walker.
func walk(r io.Reader, w walker, flags byte) (n int64, err error) {
	s := stream.Wrap(r)
	start := s.Offset()
	if flags&SingleStream != 0 {
		err = walkStream(s, w, 0)
		n = s.Offset() - start
		if err != nil {
			return n, err
		}
		// check for EOF
		var a [1]byte
		if _, err = s.Read(a[:]); err != io.EOF {
			return n, errors.New(
				"xz: expected EOF at end of single stream")
		}
		return n, nil
	}

	err = walkStream(s, w, 0)
	if err != nil {
		return s.Offset() - start, err
	}
	for {
		if err = walkStream(s, w, expectPadding); err != nil {
			n = s.Offset() - start
			if err == io.EOF {
				return n, nil
			}
			return n, err
		}
	}
}

// Info provides information about an xz-compressed file.
type Info struct {
	Streams      int64
	Blocks       int64
	Uncompressed int64
	Compressed   int64
	Check        byte
}

// infoWalker collects the information about the uncompressed bytes in an xz
// stream.
type infoWalker struct {
	Info
}

func (w *infoWalker) streamHeader(h header) error {
	w.Check = h.flags & 0x0f
	w.Streams++
	return nil
}

func (w *infoWalker) streamFooter(f footer) error {
	return nil
}

func (w *infoWalker) blockHeader(bh blockHeader) error {
	w.Blocks++
	return errSuppressChunks
}

func (w *infoWalker) chunkHeader(ch lzma.ChunkHeader) error {
	return nil
}

func (w *infoWalker) index(records int) error {
	return nil
}

func (w *infoWalker) record(r record) error {
	w.Uncompressed += int64(r.uncompressedSize)
	return nil
}

// Stat provides statistics about the data in an xz file.
func Stat(r io.Reader, flags byte) (info Info, err error) {
	var w infoWalker
	n, err := walk(r, &w, flags)
	if err != nil {
		return Info{}, err
	}

	w.Compressed = n
	return w.Info, nil
}
