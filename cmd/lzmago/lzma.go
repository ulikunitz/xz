package main

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"strings"

	"github.com/uli-go/xz/lzma"
)

type reader struct {
	file *os.File
	*bufio.Reader
	stdin  bool
	remove bool
}

func newReader(path string, opts *options) (r *reader, err error) {
	if path == "-" {
		r = &reader{
			file:   os.Stdin,
			Reader: bufio.NewReader(os.Stdin),
			stdin:  true,
		}
		return r, nil
	}
	fi, err := os.Lstat(path)
	if err != nil {
		return nil, err
	}
	if !fi.Mode().IsRegular() {
		return nil, fmt.Errorf("%s is not a reqular file", path)
	}
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	r = &reader{
		file:   file,
		Reader: bufio.NewReader(file),
	}
	if !opts.keep {
		r.remove = true
	}
	return r, nil
}

var errReaderClosed = errors.New("reader already closed")

func (r *reader) Close() error {
	if r.Reader == nil {
		return errReaderClosed
	}

	var err error
	if !r.stdin {
		if err = r.file.Close(); err != nil {
			return err
		}
		if r.remove {
			if err = os.Remove(r.file.Name()); err != nil {
				return err
			}
		}
	}

	*r = reader{}
	return nil
}

func (r *reader) Cancel() error {
	if r.Reader == nil {
		return errReaderClosed
	}

	if !r.stdin {
		if err := r.file.Close(); err != nil {
			return err
		}
	}

	*r = reader{}
	return nil
}

type writer struct {
	file *os.File
	*bufio.Writer
	stdout bool
	rename bool
	name   string
}

func newWriter(path string, opts *options) (w *writer, err error) {
	if path == "-" || opts.stdout {
		w = &writer{
			file:   os.Stdout,
			Writer: bufio.NewWriter(os.Stdout),
			stdout: true,
		}
		return w, nil
	}
	const ext = ".lzma"
	var name string
	if opts.decompress {
		if !strings.HasSuffix(path, ext) {
			return nil, errors.New("unknown suffix -- file ignored")
		}
		name = path[:len(path)-len(ext)]
	} else {
		name = path + ext
	}
	var dir string
	if dir, err = os.Getwd(); err != nil {
		return nil, err
	}
	file, err := ioutil.TempFile(dir, "lzma-")
	if err != nil {
		return nil, err
	}
	w = &writer{
		file:   file,
		Writer: bufio.NewWriter(file),
		rename: true,
		name:   name,
	}
	return w, nil
}

var errWriterClosed = errors.New("writer already closed")

func (w *writer) Close() error {
	if w.Writer == nil {
		return errWriterClosed
	}

	var err error
	if err = w.Writer.Flush(); err != nil {
		return err
	}

	if !w.stdout {
		if err = w.file.Close(); err != nil {
			return err
		}
		if w.rename {
			if err = os.Rename(w.file.Name(), w.name); err != nil {
				return err
			}
		} else {
			if err = os.Remove(w.file.Name()); err != nil {
				return err
			}
		}
	}

	*w = writer{}
	return nil
}

func (w *writer) Cancel() error {
	if w.Writer == nil {
		return errWriterClosed
	}

	var err error
	if !w.stdout {
		if err = w.file.Close(); err != nil {
			return err
		}
		if err = os.Remove(w.file.Name()); err != nil {
			return err
		}
	}

	*w = writer{}
	return nil
}

type decompressor struct {
	*lzma.Reader
	r *reader
}

func newDecompressor(path string, opts *options) (d *decompressor, err error) {
	r, err := newReader(path, opts)
	if err != nil {
		return nil, err
	}
	lr, err := lzma.NewReader(r)
	if err != nil {
		r.Cancel()
		return nil, err
	}
	d = &decompressor{Reader: lr, r: r}
	return d, nil
}

func (d *decompressor) Close() error {
	d.Reader = nil
	if err := d.r.Close(); err != nil {
		return err
	}
	d.r = nil
	return nil
}

func (d *decompressor) Cancel() error {
	if d.Reader == nil {
		return nil
	}
	if err := d.r.Cancel(); err != nil {
		return err
	}
	d.Reader = nil
	d.r = nil
	return nil
}

type compressor struct {
	*lzma.Writer
	w *writer
}

// parameters converts the lzmago executable flags to lzma parameters.
//
// I cannot use the preset config from the Tukaani project directly,
// because I don't have two algorithm modes and can't support parameters
// like nice_len or depth. So at this point in time I stay with the
// dictionary sizes the default combination of (LC,LP,LB) = (3,0,2).
// The default preset is 6.
// Following list provides exponents of two for the dictionary sizes:
// 18, 20, 21, 22, 22, 23, 23, 24, 25, 26.
func parameters(opts *options) lzma.Parameters {
	dictSizeExps := []uint{18, 20, 21, 22, 22, 23, 23, 24, 25, 26}
	dictSize := int64(1) << dictSizeExps[opts.preset]
	p := lzma.Parameters{
		LC:           3,
		LP:           0,
		PB:           2,
		DictSize:     dictSize,
		EOS:          true,
		ExtraBufSize: 16 * 1024,
	}
	return p
}

func newCompressor(path string, opts *options) (c *compressor, err error) {
	w, err := newWriter(path, opts)
	if err != nil {
		return nil, err
	}
	p := parameters(opts)
	lw, err := lzma.NewWriterParams(w, p)
	if err != nil {
		w.Cancel()
		return nil, err
	}
	c = &compressor{
		Writer: lw,
		w:      w,
	}
	return c, nil
}

func (c *compressor) Close() error {
	var err error
	if err = c.Writer.Close(); err != nil {
		return err
	}
	if err := c.w.Close(); err != nil {
		return err
	}
	c.w = nil
	c.Writer = nil
	return nil
}

func (c *compressor) Cancel() error {
	if c.Writer == nil {
		return nil
	}
	if err := c.w.Cancel(); err != nil {
		return err
	}
	c.w = nil
	c.Writer = nil
	return nil
}

type readCanceler interface {
	io.ReadCloser
	Cancel() error
}

func newReadCanceler(path string, opt *options) (r readCanceler, err error) {
	if opt.decompress {
		r, err = newDecompressor(path, opt)
	} else {
		r, err = newReader(path, opt)
	}
	return
}

type writeCanceler interface {
	io.WriteCloser
	Cancel() error
}

func newWriteCanceler(path string, opt *options) (w writeCanceler, err error) {
	if !opt.decompress {
		w, err = newCompressor(path, opt)
	} else {
		w, err = newWriter(path, opt)
	}
	return
}

func processLZMA(path string, opts *options) (err error) {
	r, err := newReadCanceler(path, opts)
	if err != nil {
		return err
	}
	defer func() {
		if err != nil {
			r.Cancel()
		} else {
			err = r.Close()
		}
	}()
	w, err := newWriteCanceler(path, opts)
	if err != nil {
		return err
	}
	defer func() {
		if err != nil {
			w.Cancel()
		} else {
			err = w.Close()
		}
	}()
	for {
		_, err = io.CopyN(w, r, 64*1024)
		if err != nil {
			if err == io.EOF {
				err = nil
			}
			return
		}
	}
}
