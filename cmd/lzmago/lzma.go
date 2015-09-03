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

type packer interface {
	outputPaths(path string) (outputPath, tmpPath string, err error)
	pack(w io.Writer, r io.Reader, preset int) (n int64, err error)
}

const lzmaSuffix = ".lzma"

// parameters converts the lzmago executable flags to lzma parameters.
//
// I cannot use the preset config from the Tukaani project directly,
// because I don't have two algorithm modes and can't support parameters
// like nice_len or depth. So at this point in time I stay with the
// dictionary sizes the default combination of (LC,LP,LB) = (3,0,2).
// The default preset is 6.
// Following list provides exponents of two for the dictionary sizes:
// 18, 20, 21, 22, 22, 23, 23, 24, 25, 26.
func parameters(preset int) lzma.Parameters {
	dictSizeExps := []uint{18, 20, 21, 22, 22, 23, 23, 24, 25, 26}
	dictSize := int64(1) << dictSizeExps[preset]
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

type lzmaPacker struct{}

func (p lzmaPacker) outputPaths(path string) (out, tmp string, err error) {
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
	tmp = out + ".pack"
	return
}

func (p lzmaPacker) pack(w io.Writer, r io.Reader, preset int) (n int64, err error) {
	if w == nil {
		panic("writer w is nil")
	}
	if r == nil {
		panic("reader r is nil")
	}
	params := parameters(preset)
	bw := bufio.NewWriter(w)
	lw, err := lzma.NewWriterParams(bw, params)
	if err != nil {
		return
	}
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

type lzmaUnpacker struct{}

func (u lzmaUnpacker) outputPaths(path string) (out, tmp string, err error) {
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
	tmp = out + ".unpack"
	return
}

func (u lzmaUnpacker) pack(w io.Writer, r io.Reader, preset int) (n int64, err error) {
	if w == nil {
		panic("writer w is nil")
	}
	if r == nil {
		panic("reader r is nil")
	}
	// pack actually unpacks
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

func packFile(pck packer, path, tmpPath string, opts *options) error {
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

	_, err = pck.pack(w, r, opts.preset)
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
	var pck packer
	if opts.decompress {
		pck = lzmaUnpacker{}
	} else {
		pck = lzmaPacker{}
	}
	outputPath, tmpPath, err := pck.outputPaths(path)
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

	if err = packFile(pck, path, tmpPath, opts); err != nil {
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
