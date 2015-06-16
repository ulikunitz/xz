package main

import (
	"bufio"
	"errors"
	"fmt"
	"io/ioutil"
	"os"
)

// I cannot use the preset config from the Tukaani project directly,
// because I don't have two algorithm modes and can't support parameters
// like nice_len or depth. So at this point in time I stay with the
// dictionary sizes the default combination of (LC,LP,LB) = (3,0,2).
// The default preset is 6.
// Following list provides exponents of two for the dictionary sizes:
// 18, 20, 21, 22, 22, 23, 23, 24, 25, 26.

type reader struct {
	file *os.File
	*bufio.Reader
	stdin  bool
	remove bool
}

func newReader(opts options, arg string) (r *reader, err error) {
	if arg == "-" {
		r = &reader{
			file:   os.Stdin,
			Reader: bufio.NewReader(os.Stdin),
			stdin:  true,
		}
		return r, nil
	}
	fi, err := os.Lstat(arg)
	if err != nil {
		return nil, err
	}
	if !fi.Mode().IsRegular() {
		return nil, fmt.Errorf("%s is not a reqular file", arg)
	}
	file, err := os.Open(arg)
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

func (r *reader) kill() error {
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

func newWriter(opts options, arg string) (w *writer, err error) {
	if arg == "-" || opts.stdout {
		w = &writer{
			file:   os.Stdout,
			Writer: bufio.NewWriter(os.Stdout),
			stdout: true,
		}
		return w, nil
	}
	name := arg + ".lzma"
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

func (w *writer) kill() error {
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

func processLZMA(opts options, arg string) error {
	// TODO: signal handling
	// create buffered input reader
	// create buffered output writer
	// create lzma filter
	// copy data
	// assuming no error
	// close output
	// close input
	// rename output to correct file
	// remove input file if not kept and not stdin
	panic("TODO")
}
