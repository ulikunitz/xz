package xz

import (
	"io"
	"sync"
)

// ReaderAtConfig defines the parameters for the xz readerat.
type ReaderAtConfig struct{}

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
		conf: c,
		xz:   xz,
	}

	if err := r.init(); err != nil {
		return nil, err
	}

	return r, nil

}

func (r *ReaderAt) init() error {
	return nil
}

func (r *ReaderAt) ReadAt(p []byte, offset int64) (int, error) {
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
