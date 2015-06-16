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
	remove bool
}

func newReader(opts options, arg string) (r *reader, err error) {
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

func (r *reader) Close() error {
	var name string
	if r.file != nil {
		name = r.file.Name()
	}

	if err := r.kill(); err != nil {
		return err
	}

	if !r.remove {
		return nil
	}
	return os.Remove(name)
}

func (r *reader) kill() error {
	if r.Reader == nil {
		return errors.New("reader already closed")
	}

	if err := r.file.Close(); err != nil {
		return err
	}

	r.Reader = nil
	r.file = nil
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
	w = new(writer)
	if opts.stdout {
		w.stdout = true
		w.file = os.Stdout
	} else {
		if arg == "-" {
			return nil, errors.New(
				"argument '-' requires -c flag for standard output")
		}
		w.name = arg + ".lzma"
		var dir string
		if dir, err = os.Getwd(); err != nil {
			return nil, err
		}
		if w.file, err = ioutil.TempFile(dir, "lzma-"); err != nil {
			return nil, err
		}
		w.rename = true
	}
	w.Writer = bufio.NewWriter(w.file)
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

	w.file = nil
	w.Writer = nil
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

	w.file = nil
	w.Writer = nil
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
