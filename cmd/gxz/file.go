// Copyright 2015 Ulrich Kunitz. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"github.com/ulikunitz/xz"
	"github.com/ulikunitz/xz/internal/xlog"
	"github.com/ulikunitz/xz/lzma"
)

// signalHandler establishes the signal handler for SIGTERM(1) and
// handles it in its own go routine. The returned quit channel must be
// closed to terminate the signal handler go routine.
func signalHandler(w *writer) chan<- struct{} {
	quit := make(chan struct{})
	sigch := make(chan os.Signal, 1)
	signal.Notify(sigch, os.Interrupt)
	go func() {
		select {
		case <-quit:
			signal.Stop(sigch)
			return
		case <-sigch:
			w.removeTmpFile()
			os.Exit(7)
		}
	}()
	return quit
}

// format defines the newCompressor and newDecompressor functions for a
// compression format.
type format struct {
	newCompressor func(w io.Writer, opts *options) (c io.WriteCloser,
		err error)
	newDecompressor func(r io.Reader, opts *options) (d io.Reader,
		err error)
}

// dictCapExps maps preset values to exponent for dictionary capacity
// sizes.
var lzmaDictCapExps = []uint{18, 20, 21, 22, 22, 23, 23, 24, 25, 26}

// formats contains the formats supported by gxz.
var formats = map[string]*format{
	"lzma": &format{
		newCompressor: func(w io.Writer, opts *options,
		) (c io.WriteCloser, err error) {
			lz := lzma.NewWriter(w)
			lz.Properties = lzma.Properties{LC: 3, LP: 0, PB: 2}
			lz.DictCap = 1 << lzmaDictCapExps[opts.preset]
			lz.Size = -1
			lz.EOSMarker = true
			return lz, nil
		},
		newDecompressor: func(r io.Reader, opts *options,
		) (d io.Reader, err error) {
			lz, err := lzma.NewReader(r)
			if err != nil {
				return nil, err
			}
			lz.DictCap = 1 << lzmaDictCapExps[opts.preset]
			return lz, err
		},
	},
	"xz": &format{
		newCompressor: func(w io.Writer, opts *options,
		) (c io.WriteCloser, err error) {
			p := xz.WriterDefaults
			p.DictCap = 1 << lzmaDictCapExps[opts.preset]
			return xz.NewWriterParams(w, &p)
		},
		newDecompressor: func(r io.Reader, opts *options,
		) (d io.Reader, err error) {
			p := xz.ReaderDefaults
			p.DictCap = 1 << lzmaDictCapExps[opts.preset]
			return xz.NewReaderParams(r, &p)
		},
	},
}

// targetName finds the correct target name taking the options into
// account.
func targetName(path string, opts *options) (target string, err error) {
	if path == "-" {
		panic("path name - not supported")
	}
	if len(path) == 0 {
		return "", errors.New("empty file name not supported")
	}
	ext := "." + opts.format
	if !opts.decompress {
		return path + ext, nil
	}
	if strings.HasSuffix(path, ext) {
		target = path[:len(path)-len(ext)]
		if len(target) == 0 {
			return "", fmt.Errorf(
				"file name %s has no base part", path)
		}
		return target, nil
	}
	return path, nil
}

// tmpName converts the path string into a temporary name by appending
// .decompress or .compress to the file path.
func tmpName(path string, decompress bool) string {
	var ext string
	if decompress {
		ext = ".decompress"
	} else {
		ext = ".compress"
	}
	return path + ext
}

// writer is used as file writer for decompression and file compressor
// for compression.
type writer struct {
	f    *os.File
	name string
	bw   *bufio.Writer
	io.Writer
	cmp     io.WriteCloser
	success bool
}

// writerFormat select the writer format.
func writerFormat(opts *options) (f *format, err error) {
	var ok bool
	if f, ok = formats[opts.format]; !ok {
		return nil, fmt.Errorf("compression format %q not supported",
			opts.format)
	}
	return f, nil
}

// newCompressor creates a compressor for the given writer.
func newCompressor(w io.Writer, opts *options) (cmp io.WriteCloser, err error) {
	if opts.decompress {
		panic("no compressor needed")
	}
	f, err := writerFormat(opts)
	if err != nil {
		return nil, err
	}
	if cmp, err = f.newCompressor(w, opts); err != nil {
		return nil, err
	}
	return cmp, nil
}

// newWriter creates a new file writer. Note that options must contain
// the actual compression format supported and not just auto.
func newWriter(path string, perm os.FileMode, opts *options,
) (w *writer, err error) {
	w = &writer{name: path}
	if opts.stdout {
		w.f = os.Stdout
		w.name = "-"
	} else {
		name, err := targetName(path, opts)
		if err != nil {
			return nil, err
		}
		if _, err = os.Stat(name); !os.IsNotExist(err) {
			if !opts.force {
				return nil, &userPathError{
					Path: name,
					Err:  errors.New("file exists")}
			}
			if err = os.Remove(name); err != nil {
				return nil, err
			}
		}
		tmp := tmpName(path, opts.decompress)
		if w.f, err = os.OpenFile(tmp,
			os.O_WRONLY|os.O_CREATE|os.O_EXCL, perm); err != nil {
			return nil, err
		}
		w.name = name
	}
	w.bw = bufio.NewWriter(w.f)
	if opts.decompress {
		w.Writer = w.bw
		return w, nil
	}
	w.cmp, err = newCompressor(w.bw, opts)
	if err != nil {
		return nil, err
	}
	w.Writer = w.cmp
	return w, nil
}

// isStdout checks whether the parameter refers to stdout.
func isStdout(f *os.File) bool {
	return f.Fd() == uintptr(syscall.Stdout)
}

var errInval = errors.New("invalid value")

// Close closes the writer. Note that the behaviour depends whether
// success has been set for the writer.
func (w *writer) Close() error {
	var err error

	if w.f == nil {
		return errInval
	}
	defer func() { w.f = nil }()

	if !w.success {
		if isStdout(w.f) {
			return nil
		}
		if err = w.f.Close(); err != nil {
			return err
		}
		if err = os.Remove(w.f.Name()); err != nil {
			return err
		}
		return nil
	}
	if w.cmp != nil {
		if err = w.cmp.Close(); err != nil {
			return err
		}
	}
	if err = w.bw.Flush(); err != nil {
		return err
	}
	if isStdout(w.f) {
		return nil
	}
	if err = w.f.Close(); err != nil {
		return err
	}
	if err = os.Rename(w.f.Name(), w.name); err != nil {
		return err
	}
	return nil
}

// removeTmpFile removes the temporary file for the writer. It is used
// by the signal handler goroutine.
func (w *writer) removeTmpFile() {
	os.Remove(w.f.Name())
}

// SetSuccess sets the success variable to true.
func (w *writer) SetSuccess() { w.success = true }

// reader is used as a file reader.
type reader struct {
	f *os.File
	io.Reader
	success bool
	keep    bool
}

// errNoRefular indicates that a file is not regular.
var errNoRegular = errors.New("no regular file")

// specialBits contain the special bits, which are not supported by gxz.
const specialBits = os.ModeSetuid | os.ModeSetgid | os.ModeSticky

// openFile opens the given path with the given options.
func openFile(path string, opts *options) (f *os.File, err error) {
	if path == "-" {
		return os.Stdin, nil
	}
	fi, err := os.Lstat(path)
	if err != nil {
		return nil, err
	}
	fm := fi.Mode()
	if !fm.IsRegular() {
		if !opts.force || fm&os.ModeSymlink == 0 {
			return nil, &userPathError{Path: path,
				Err: errNoRegular}
		}
	}
	if f, err = os.Open(path); err != nil {
		return nil, err
	}
	if fi, err = f.Stat(); err != nil {
		return nil, err
	}
	fm = fi.Mode()
	if !fm.IsRegular() {
		return nil, &userPathError{Path: path, Err: errNoRegular}
	}
	if fm&specialBits != 0 && !opts.force {
		return nil, &userPathError{Path: path,
			Err: errors.New("setuid, setgid and/or sticky bit set")}
	}
	return f, nil
}

// readerFormat tries to determine the type of a file. Currently it
// checks for the XZ header magic and if it is not present assumes that
// the file has been encoded by LZMA. The format field in options is
// updated.
func readerFormat(br *bufio.Reader, opts *options) (f *format, err error) {
	var ok bool
	if f, ok = formats[opts.format]; ok {
		return f, nil
	}
	if opts.format != "auto" {
		return nil, errors.New("compression format not supported")
	}
	var headerMagic = []byte{0xfd, '7', 'z', 'X', 'Z', 0x00}
	p, err := br.Peek(len(headerMagic))
	if err != nil {
		return nil, err
	}
	if bytes.Equal(p, headerMagic) {
		opts.format = "xz"
	} else {
		opts.format = "lzma"
	}
	return formats[opts.format], nil
}

// newDecompressor creates a new decompressor.
func newDecompressor(br *bufio.Reader, opts *options) (dec io.Reader,
	err error) {
	if !opts.decompress {
		panic("no decompressor needed")
	}
	f, err := readerFormat(br, opts)
	if err != nil {
		return nil, err
	}
	if dec, err = f.newDecompressor(br, opts); err != nil {
		return nil, err
	}
	return dec, nil
}

// newReader creates a new reader for files.
func newReader(path string, opts *options) (r *reader, err error) {
	f, err := openFile(path, opts)
	if err != nil {
		return nil, err
	}
	br := bufio.NewReader(f)
	if !opts.decompress {
		r = &reader{f: f, Reader: br, keep: opts.keep || opts.stdout}
		return r, nil
	}
	dec, err := newDecompressor(br, opts)
	if err != nil {
		return nil, err
	}
	r = &reader{f: f, Reader: dec, keep: opts.keep || opts.stdout}
	return r, nil
}

// isStdin checks whether the given file reference is stdin.
func isStdin(f *os.File) bool {
	return f.Fd() == uintptr(syscall.Stdin)
}

// Close closes the reader. The behaviour can be influences by the
// success attribute of reader.
func (r *reader) Close() error {
	if r.f == nil {
		return errInval
	}
	defer func() { r.f = nil }()
	if isStdin(r.f) {
		return nil
	}
	if err := r.f.Close(); err != nil {
		return err
	}
	if r.keep || !r.success {
		return nil
	}
	if err := os.Remove(r.f.Name()); err != nil {
		return err
	}
	return nil
}

func (r *reader) SetSuccess() { r.success = true }

func (r *reader) Perm() os.FileMode {
	const defaultPerm os.FileMode = 0666

	fi, err := r.f.Stat()
	if err != nil {
		return defaultPerm
	}

	return fi.Mode() & defaultPerm
}

// userPathError represents a path error presentable to a user. In
// difference to os.PathError it removes the information of the
// operation returning the error.
type userPathError struct {
	Path string
	Err  error
}

// Error provides the error string for the path error.
func (e *userPathError) Error() string {
	return e.Path + ": " + e.Err.Error()
}

// userError converts path error to an error message that is
// acceptable for gxz users. PathError provides information about the
// command that has created an error. For instance Lstat informs that
// lstat detected that a file didn't exist this information is not
// relevant for users of the gxz program. This function converts a
// path error into a generic error removing the operation information.
func userError(err error) error {
	pe, ok := err.(*os.PathError)
	if !ok {
		return err
	}
	return &userPathError{Path: pe.Path, Err: pe.Err}
}

func printErr(err error) {
	if err != nil {
		xlog.Warn(userError(err))
	}
}

// processFile process the file with the given path applying the
// provided options.
func processFile(path string, opts *options) (err error) {
	r, err := newReader(path, opts)
	if err != nil {
		printErr(err)
		return
	}
	defer r.Close()
	w, err := newWriter(path, r.Perm(), opts)
	if err != nil {
		printErr(err)
		return
	}
	defer w.Close()
	quitSignalHandler := signalHandler(w)
	if _, err = io.Copy(w, r); err != nil {
		close(quitSignalHandler)
		printErr(err)
		return err
	}
	close(quitSignalHandler)
	w.SetSuccess()
	if err = w.Close(); err != nil {
		printErr(err)
		return err
	}
	r.SetSuccess()
	if err = r.Close(); err != nil {
		printErr(err)
		return err
	}
	return nil
}
