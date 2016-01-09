// Copyright 2015 Ulrich Kunitz. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"os"
	"os/signal"
	"path/filepath"
	"strings"

	"github.com/ulikunitz/xz/lzma"
	"github.com/ulikunitz/xz/xlog"
)

type compressor interface {
	outputPaths(path string) (outputPath, tmpPath string, err error)
	compress(w io.Writer, r io.Reader, preset int) (n int64, err error)
}

const lzmaSuffix = ".lzma"

// dictCapExps maps preset values to exponent for dictionary capacity
// sizes.
var dictCapExps = []uint{18, 20, 21, 22, 22, 23, 23, 24, 25, 26}

// setParameters sets the parameters for the lzma writer using the given
// preset.
func setParameters(w *lzma.Writer, preset int) {
	w.Properties = lzma.Properties{LC: 3, LP: 0, PB: 2}
	w.DictCap = 1 << dictCapExps[preset]
	w.Size = -1
	w.EOSMarker = true
}

type lzmaCompressor struct{}

func (p lzmaCompressor) outputPaths(path string) (out, tmp string, err error) {
	if path == "-" {
		return "-", "-", nil
	}
	if path == "" {
		err = errors.New("path is empty")
		return
	}
	if strings.HasSuffix(path, lzmaSuffix) {
		err = fmt.Errorf("path %s has suffix %s -- ignored",
			path, lzmaSuffix)
		return
	}
	out = path + lzmaSuffix
	tmp = out + ".compress"
	return
}

func (p lzmaCompressor) compress(w io.Writer, r io.Reader, preset int) (n int64, err error) {
	if w == nil {
		panic("writer w is nil")
	}
	if r == nil {
		panic("reader r is nil")
	}
	bw := bufio.NewWriter(w)
	lw := lzma.NewWriter(bw)
	setParameters(lw, preset)
	n, err = io.Copy(lw, r)
	if err != nil {
		return
	}
	if err = lw.Close(); err != nil {
		return
	}
	err = bw.Flush()
	return
}

type lzmaDecompressor struct{}

func (d lzmaDecompressor) outputPaths(path string) (out, tmp string, err error) {
	if path == "-" {
		return "-", "-", nil
	}
	if !strings.HasSuffix(path, lzmaSuffix) {
		err = fmt.Errorf("path %s has no suffix %s",
			path, lzmaSuffix)
		return
	}
	base := filepath.Base(path)
	if base == lzmaSuffix {
		err = fmt.Errorf(
			"path %s has only suffix %s as filename",
			path, lzmaSuffix)
		return
	}
	out = path[:len(path)-len(lzmaSuffix)]
	tmp = out + ".decompress"
	return
}

func (u lzmaDecompressor) compress(w io.Writer, r io.Reader, preset int) (n int64, err error) {
	if w == nil {
		panic("writer w is nil")
	}
	if r == nil {
		panic("reader r is nil")
	}
	br := bufio.NewReader(r)
	lr, err := lzma.NewReader(br)
	if err != nil {
		return
	}
	n, err = io.Copy(w, lr)
	return
}

func signalHandler(tmpPath string) chan<- struct{} {
	quit := make(chan struct{})
	sigch := make(chan os.Signal, 1)
	signal.Notify(sigch, os.Interrupt)
	go func() {
		select {
		case <-quit:
			signal.Stop(sigch)
			return
		case <-sigch:
			if tmpPath != "-" {
				os.Remove(tmpPath)
			}
			os.Exit(7)
		}
	}()
	return quit
}

func compressFile(comp compressor, path, tmpPath string, opts *options) error {
	var err error

	// open reader
	var r *os.File
	if path == "-" {
		r = os.Stdin
	} else {
		fi, err := os.Lstat(path)
		if err != nil {
			return err
		}
		if !fi.Mode().IsRegular() {
			return fmt.Errorf("%s is not a regular file", path)
		}
		r, err = os.Open(path)
		if err != nil {
			return err
		}
		fi, err = r.Stat()
		if err != nil {
			r.Close()
			return err
		}
		if !fi.Mode().IsRegular() {
			r.Close()
			return fmt.Errorf("%s is not a regular file", path)
		}
	}
	defer func() {
		if err != nil {
			r.Close()
		} else {
			err = r.Close()
		}
	}()

	// open writer
	var w *os.File
	if tmpPath == "-" {
		w = os.Stdout
	} else {
		if opts.force {
			os.Remove(tmpPath)
		}
		w, err = os.OpenFile(tmpPath,
			os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0666)
		if err != nil {
			return err
		}
		defer func() {
			if err != nil {
				w.Close()
			} else {
				err = w.Close()
			}
		}()
		fi, err := w.Stat()
		if err != nil {
			return err
		}
		if !fi.Mode().IsRegular() {
			return fmt.Errorf("%s is not a regular file", tmpPath)
		}
	}

	_, err = comp.compress(w, r, opts.preset)
	return err
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
// acceptable for lzmago users. PathError provides information about the
// command that has created an error. For instance Lstat informs that
// lstat detected that a file didn't exist this information is not
// relevant for users of the lzmago program. This function converts a
// path error into a generic error removing the operation information.
func userError(err error) error {
	pe, ok := err.(*os.PathError)
	if !ok {
		return err
	}
	return &userPathError{Path: pe.Path, Err: pe.Err}
}

func processFile(path string, opts *options) {
	var comp compressor
	if opts.decompress {
		comp = lzmaDecompressor{}
	} else {
		comp = lzmaCompressor{}
	}
	outputPath, tmpPath, err := comp.outputPaths(path)
	if err != nil {
		xlog.Warn(userError(err))
		return
	}
	if opts.stdout {
		outputPath, tmpPath = "-", "-"
	}
	if outputPath != "-" {
		_, err = os.Lstat(outputPath)
		if err == nil && !opts.force {
			xlog.Warnf("file %s exists", outputPath)
			return
		}
	}
	defer func() {
		if tmpPath != "-" {
			os.Remove(tmpPath)
		}
	}()
	quit := signalHandler(tmpPath)
	defer close(quit)

	if err = compressFile(comp, path, tmpPath, opts); err != nil {
		xlog.Warn(userError(err))
		return
	}
	if tmpPath != "-" && outputPath != "-" {
		if err = os.Rename(tmpPath, outputPath); err != nil {
			xlog.Warn(userError(err))
			return
		}
	}
	if !opts.keep && !opts.stdout && path != "-" {
		if err = os.Remove(path); err != nil {
			xlog.Warn(userError(err))
			return
		}
	}
}
