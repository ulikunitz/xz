package xz

import (
	"fmt"
	"io"
	"sync"

	"github.com/ulikunitz/xz/internal/xlog"
)

// ReaderAtConfig defines the parameters for the xz readerat.
type ReaderAtConfig struct {
	Len int64
}

// Verify checks the reader config for validity. Zero values will be replaced by
// default values.
func (c *ReaderAtConfig) Verify() error {
	// if c == nil {
	// 	return errors.New("xz: reader parameters are nil")
	// }
	return nil
}

// ReaderAt supports the reading of one or multiple xz streams.
type ReaderAt struct {
	conf ReaderAtConfig

	indices []index

	// len of the contents of the underlying xz data
	len int64
	xz  io.ReaderAt
}

// NewReader creates a new xz reader using the default parameters.
// The function reads and checks the header of the first XZ stream. The
// reader will process multiple streams including padding.
func NewReaderAt(xz io.ReaderAt) (r *ReaderAt, err error) {
	return ReaderAtConfig{}.NewReaderAt(xz)
}

// NewReaderAt creates an xz stream reader.
func (c ReaderAtConfig) NewReaderAt(xz io.ReaderAt) (*ReaderAt, error) {
	if err := c.Verify(); err != nil {
		return nil, err
	}

	r := &ReaderAt{
		conf:    c,
		len:     0,
		indices: []index{},
		xz:      xz,
	}

	r.len = r.conf.Len
	if r.len < 1 {
		panic("todo: implement probing for Len")
	}

	streamEnd := r.len - 1

	for streamEnd > 0 {
		streamStart, err := r.setupIndexAt(streamEnd)
		if err != nil {
			return nil, fmt.Errorf("trouble creating indices: %v", err)
		}

		// the end of the next stream reading backwards is one before the start
		// of the one we just processed.
		streamEnd = streamStart - 1
	}

	return r, nil
}

// An index carries all the information necessary for reading randomly into a
// single stream.
type index struct {
	blockStartOffset int64
	streamHeader     streamHeader
	records          []record
}

func (i index) compressedBufferedSize() int64 {
	size := int64(0)
	for _, r := range i.records {
		size += r.paddedLen()
	}
	return size
}

// setupIndexAt takes the offset of the end of a stream, or null bytes following
// the end of a stream. It builds an index for that stream, adds it to the
// beginning of the ReaderAt and returns the offset to the beginning of the stream.
func (r *ReaderAt) setupIndexAt(endOffset int64) (int64, error) {
	// read backwards past potential null bytes until we find the end of the
	// footer
	for endOffset > 0 {
		probe := make([]byte, 1)
		n, err := r.xz.ReadAt(probe, endOffset)
		if err != nil {
			return 0, err
		}
		if n != len(probe) {
			return 0, fmt.Errorf("read %d bytes", n)
		}
		if probe[0] != 0 {
			break
		}
		endOffset--
	}
	endOffset++

	footerOffset := endOffset - footerLen
	f, err := readFooter(newRat(r.xz, footerOffset))
	if err != nil {
		return 0, err
	}

	indexStartOffset := footerOffset - f.indexSize

	// readIndexBody assumes the indicator byte has already been read
	indexRecs, _, err := readIndexBody(newRat(r.xz, indexStartOffset+1))
	if err != nil {
		return 0, err
	}

	ix := index{
		records: indexRecs,
	}
	ix.blockStartOffset = indexStartOffset - ix.compressedBufferedSize()
	r.indices = append([]index{ix}, r.indices...)

	sh := streamHeader{}
	headerStartOffset := ix.blockStartOffset - HeaderLen
	err = sh.UnmarshalReader(newRat(r.xz, headerStartOffset))
	if err != nil {
		return 0, fmt.Errorf("trouble reading stream header at offset %d: %v", headerStartOffset, err)
	}
	ix.streamHeader = sh

	xlog.Debugf("xz indices %+v", r.indices)

	return headerStartOffset, nil
}

func (r *ReaderAt) ReadAt(p []byte, bufferPos int64) (int, error) {
	lenRequested := len(p)

	indicesPos := int64(0)

	for _, index := range r.indices {
		blockOffset := index.blockStartOffset

		for _, block := range index.records {
			if indicesPos <= bufferPos && bufferPos <= indicesPos+block.uncompressedSize {
				blockStartPos := bufferPos - indicesPos
				blockEndPos := blockStartPos + int64(len(p))
				if blockEndPos > block.uncompressedSize {
					blockEndPos = block.uncompressedSize
				}
				blockAmtToRead := blockEndPos - blockStartPos

				r.readBlockAt(
					p[:blockAmtToRead], blockStartPos,
					blockOffset, block.unpaddedSize, index.streamHeader.flags)
				p = p[blockAmtToRead:]
				bufferPos += blockAmtToRead
			}

			blockOffset += block.paddedLen()
			indicesPos += block.uncompressedSize
		}
	}

	var err error
	if len(p) != 0 {
		err = io.EOF
	}
	return lenRequested - len(p), err
}

func (r *ReaderAt) readBlockAt(
	p []byte, bufferPos int64,
	blockOffset, blockLen int64, streamFlags byte,
) error {
	viewStart := rat{
		Mutex:  &sync.Mutex{},
		offset: blockOffset,
		reader: r.xz,
	}

	view := io.LimitReader(&viewStart, blockLen)

	blockHeader, hlen, err := readBlockHeader(view)
	if err != nil {
		return err
	}

	readerConfig := ReaderConfig{}

	hashFn, err := newHashFunc(streamFlags)
	if err != nil {
		return err
	}
	blockReader, err := readerConfig.newBlockReader(view, blockHeader, hlen, hashFn())

	trash := make([]byte, bufferPos)
	_, err = io.ReadFull(blockReader, trash)
	if err != nil {
		return err
	}

	_, err = io.ReadFull(blockReader, p)
	return err
}

// rat wraps a ReaderAt to fulfill the io.Reader interface.
type rat struct {
	*sync.Mutex
	offset int64
	reader io.ReaderAt
}

func (r *rat) Read(p []byte) (int, error) {
	r.Lock()
	defer r.Unlock()

	n, err := r.reader.ReadAt(p, r.offset)
	r.offset += int64(n)
	return n, err
}

func newRat(ra io.ReaderAt, offset int64) *rat {
	return &rat{
		Mutex:  &sync.Mutex{},
		offset: offset,
		reader: ra,
	}
}
