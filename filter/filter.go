package filter

import (
	"errors"
	"io"

	"github.com/ulikunitz/xz/lzma"
)

// ReaderConfig defines the parameters for the xz reader. The
// SingleStream parameter requests the reader to assume that the
// underlying stream contains only a single stream.
type ReaderConfig struct {
	DictCap int
}

// WriterConfig defines the configuration parameter for a writer.
type WriterConfig struct {
	Properties *lzma.Properties
	DictCap    int
	BufSize    int

	// match algorithm
	Matcher lzma.MatchAlgorithm
}

// Filter represents a filter in the block header.
type Filter interface {
	ID() uint64
	UnmarshalBinary(data []byte) error
	MarshalBinary() (data []byte, err error)
	Reader(r io.Reader, c *ReaderConfig) (fr io.Reader, err error)
	WriteCloser(w io.WriteCloser, c *WriterConfig) (fw io.WriteCloser, err error)
	// filter must be last filter
	last() bool
}

func NewFilterReader(c *ReaderConfig, r io.Reader, f []Filter) (fr io.Reader,
	err error) {

	if err = VerifyFilters(f); err != nil {
		return nil, err
	}

	fr = r
	for i := len(f) - 1; i >= 0; i-- {
		fr, err = f[i].Reader(fr, c)
		if err != nil {
			return nil, err
		}
	}
	return fr, nil
}

// newFilterWriteCloser converts a filter list into a WriteCloser that
// can be used by a blockWriter.
func NewFilterWriteCloser(filterWriteConfig *WriterConfig, w io.Writer, f []Filter) (fw io.WriteCloser, err error) {

	if err = VerifyFilters(f); err != nil {
		return nil, err
	}
	fw = nopWriteCloser(w)
	for i := len(f) - 1; i >= 0; i-- {
		fw, err = f[i].WriteCloser(fw, filterWriteConfig)
		if err != nil {
			return nil, err
		}
	}
	return fw, nil
}

// VerifyFilters checks the filter list for the length and the right
// sequence of filters.
func VerifyFilters(f []Filter) error {
	if len(f) == 0 {
		return errors.New("xz: no filters")
	}
	if len(f) > 4 {
		return errors.New("xz: more than four filters")
	}
	for _, g := range f[:len(f)-1] {
		if g.last() {
			return errors.New("xz: last filter is not last")
		}
	}
	if !f[len(f)-1].last() {
		return errors.New("xz: wrong last filter")
	}
	return nil
}

// nopWCloser implements a WriteCloser with a Close method not doing
// anything.
type nopWCloser struct {
	io.Writer
}

// Close returns nil and doesn't do anything else.
func (c nopWCloser) Close() error {
	return nil
}

// nopWriteCloser converts the Writer into a WriteCloser with a Close
// function that does nothing beside returning nil.
func nopWriteCloser(w io.Writer) io.WriteCloser {
	return nopWCloser{w}
}
