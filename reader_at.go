package xz

import (
	"io"
	"log"
	"sync"
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

	// len of the contents of the underlying xz data
	len     int64
	indices []index

	xz io.ReaderAt
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

	if err := r.init(); err != nil {
		return nil, err
	}

	return r, nil

}

type index struct {
	startOffset int64
	rs          []record
}

func (i index) compressedBufferedSize() int64 {
	size := int64(0)
	for _, r := range i.rs {
		unpadded := r.unpaddedSize
		padded := 4 * (unpadded / 4)
		if unpadded < padded {
			padded += 4
		}

		size += padded
	}
	return size
}

func (r *ReaderAt) init() error {
	r.len = r.conf.Len
	if r.len < 1 {
		panic("todo: implement probing for Len")
	}

	footerOffset := r.len - footerLen
	f, err := readFooter(newRat(r.xz, footerOffset))
	if err != nil {
		return err
	}

	indexOffset := footerOffset - f.indexSize
	indexOffset++ // readIndexBody assumes the indicator byte has already been read
	indexRecs, _, err := readIndexBody(newRat(r.xz, indexOffset))
	if err != nil {
		return err
	}

	ix := index{
		rs: indexRecs,
	}
	ix.startOffset = indexOffset - ix.compressedBufferedSize()
	r.indices = append(r.indices, ix)

	return nil
}

func (r *ReaderAt) ReadAt(p []byte, offset int64) (int, error) {
	log.Fatal(r)
	return 1, io.EOF
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
