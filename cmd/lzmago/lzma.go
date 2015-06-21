package main

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"sync"

	"github.com/uli-go/xz/lzma"
	"github.com/uli-go/xz/xlog"
)

const ext = ".lzma"

var errInvalidOp = errors.New("invalid operation")

type canceler interface {
	Cancel() error
}

type readCanceler interface {
	io.ReadCloser
	canceler
}

type reader struct {
	io.Reader
	file   *os.File
	remove bool
}

func (r *reader) Cancel() error { r.remove = false; return nil }

func (r *reader) Close() error {
	if r.file == nil {
		return errInvalidOp
	}
	if r.file == os.Stdin {
		if r.remove {
			panic("remove for stdin")
		}
		r.file = nil
		return nil
	}
	err := r.file.Close()
	if err != nil {
		return err
	}
	if r.remove {
		if err := os.Remove(r.file.Name()); err != nil {
			return err
		}
	}
	r.file = nil
	return nil
}

func openFile(path string) (f *os.File, err error) {
	fi, err := os.Lstat(path)
	if err != nil {
		if os.IsNotExist(err) {
			err = fmt.Errorf("file %s doesn't exist", path)
		}
		return nil, err
	}
	if !fi.Mode().IsRegular() {
		return nil, fmt.Errorf("%s is not a regular file", path)
	}
	return os.Open(path)
}

func openUncompressedFile(path string) (f *os.File, err error) {
	if strings.HasSuffix(path, ext) {
		return nil, fmt.Errorf("%s has already %s suffix -- unchanged",
			path, ext)
	}
	return openFile(path)
}

func openCompressedFile(path string) (f *os.File, err error) {
	if !strings.HasSuffix(path, ext) {
		return nil, fmt.Errorf("%s has no %s suffix -- unchanged",
			path, ext)
	}
	return openFile(path)
}

func newReader(file *os.File, remove bool) (r *reader, err error) {
	br := bufio.NewReader(file)
	r = &reader{file: file, Reader: br, remove: remove}
	return r, nil
}

func newDecompressor(file *os.File, remove bool) (r *reader, err error) {
	lr, err := lzma.NewReader(bufio.NewReader(file))
	if err != nil {
		return nil, err
	}
	r = &reader{file: file, Reader: lr, remove: remove}
	return r, nil
}

func newReaderPathOpts(path string, opts *options) (r *reader, err error) {
	var file *os.File
	remove := false
	if path == "-" {
		file = os.Stdin
	} else {
		if opts.decompress {
			file, err = openCompressedFile(path)
		} else {
			file, err = openUncompressedFile(path)
		}
		if err != nil {
			return nil, err
		}
		remove = !opts.stdout && !opts.keep
	}
	if opts.decompress {
		r, err = newDecompressor(file, remove)
	} else {
		r, err = newReader(file, remove)
	}
	return
}

type outputFile struct {
	newName string
	*os.File
}

func (f *outputFile) Cancel() error {
	if f.File == nil || f.File == os.Stdout {
		return nil
	}
	f.newName = ""
	return os.Remove(f.File.Name())
}

func (f *outputFile) Close() error {
	if f.File == nil {
		return errInvalidOp
	}
	if f.File == os.Stdout {
		*f = outputFile{}
		return nil
	}
	err := f.File.Close()
	if err != nil {
		return err
	}
	if f.newName != "" {
		if err = os.Rename(f.File.Name(), f.newName); err != nil {
			return err
		}
	}
	*f = outputFile{}
	return nil
}

func createOutputFile(newName string) (f *outputFile, err error) {
	if _, err := os.Lstat(newName); err == nil {
		return nil, &os.PathError{
			Op:   "createOutputFile",
			Path: newName,
			Err:  os.ErrExist,
		}
	}
	dir := filepath.Dir(newName)
	file, err := ioutil.TempFile(dir, "lzmago-")
	if err != nil {
		return nil, err
	}
	f = &outputFile{newName: newName, File: file}
	return f, nil
}

func createUncompressedFile(path string) (f *outputFile, err error) {
	if !strings.HasSuffix(path, ext) {
		return nil, fmt.Errorf(
			"path %s has no suffix %s -- ignored", path, ext)
	}
	newName := path[:len(path)-len(ext)]
	if newName == "" {
		return nil, fmt.Errorf(
			"path contains only the suffix %s", ext)
	}
	return createOutputFile(newName)
}

func createCompressedFile(path string) (f *outputFile, err error) {
	if strings.HasSuffix(path, ext) {
		return nil, fmt.Errorf(
			"path %s has suffix %s -- ignored", path, ext)
	}
	if path == "" {
		return nil, fmt.Errorf("empty path -- ignored")
	}
	return createOutputFile(path + ext)
}

type writeCanceler interface {
	io.WriteCloser
	canceler
}

type writer struct {
	ofile *outputFile
	bw    *bufio.Writer
	io.WriteCloser
}

func (w *writer) Close() error {
	if w.ofile == nil {
		return errInvalidOp
	}
	var err error
	if err = w.WriteCloser.Close(); err != nil {
		return err
	}
	if err = w.bw.Flush(); err != nil {
		return err
	}
	if err = w.ofile.Close(); err != nil {
		return nil
	}
	w.ofile = nil
	return nil
}

func (w *writer) Cancel() error {
	if w.ofile == nil {
		return nil
	}
	return w.ofile.Cancel()
}

type nopWriteCloser struct {
	io.Writer
}

func (w nopWriteCloser) Close() error { return nil }

func newWriter(ofile *outputFile) (w *writer, err error) {
	bw := bufio.NewWriter(ofile)
	w = &writer{
		ofile:       ofile,
		bw:          bw,
		WriteCloser: nopWriteCloser{bw},
	}
	return w, nil
}

func newCompressor(ofile *outputFile, p lzma.Parameters) (w *writer, err error) {
	bw := bufio.NewWriter(ofile)
	lw, err := lzma.NewWriterParams(bw, p)
	if err != nil {
		return nil, err
	}
	w = &writer{
		ofile:       ofile,
		bw:          bw,
		WriteCloser: lw,
	}
	return w, nil
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

func newWriterPathOpts(path string, opts *options) (w *writer, err error) {
	var ofile *outputFile
	if opts.stdout || path == "-" {
		ofile = &outputFile{File: os.Stdout}
	} else {
		if opts.decompress {
			ofile, err = createUncompressedFile(path)
		} else {
			ofile, err = createCompressedFile(path)
		}
		if err != nil {
			return nil, err
		}
	}
	if opts.decompress {
		w, err = newWriter(ofile)
	} else {
		p := parameters(opts.preset)
		w, err = newCompressor(ofile, p)
	}
	return
}

type state int

const (
	sInit state = iota
	sOpen
	sClosed
	sCanceled
	sOpenCanceled
)

var stateNames = [...]string{
	sInit:         "INIT",
	sOpen:         "OPEN",
	sClosed:       "CLOSED",
	sCanceled:     "CANCELED",
	sOpenCanceled: "OPEN-CANCELED",
}

func (s state) String() string {
	if !(sInit <= s && s <= sOpenCanceled) {
		return fmt.Sprintf("UNKNOWN(%d)", s)
	}
	return stateNames[s]
}

type closeCanceler interface {
	io.Closer
	canceler
}

type stream struct {
	mu    sync.Mutex
	state state
	cc    closeCanceler
}

func (s *stream) Cancel() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	switch s.state {
	case sOpen:
		err := s.cc.Cancel()
		s.state = sOpenCanceled
		return err
	case sClosed, sCanceled:
		return nil
	default:
		s.state = sCanceled
		return nil
	}
}

func (s *stream) openCC(cc closeCanceler) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if cc == nil {
		return errors.New("Open argument is nil")
	}
	if s.state != sInit {
		return fmt.Errorf("Open in state %s", s.state)
	}
	s.cc = cc
	s.state = sOpen
	return nil
}

func (s *stream) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	switch s.state {
	case sClosed:
		return fmt.Errorf("Close in state %s", s.state)
	case sOpen, sOpenCanceled:
		err := s.cc.Close()
		s.cc = nil
		s.state = sClosed
		return err
	default:
		s.state = sClosed
		return nil
	}
}

type input struct {
	stream
	r *reader
}

func (in *input) Open(r *reader) error {
	if err := in.openCC(r); err != nil {
		return err
	}
	in.r = r
	return nil
}

func (in *input) Read(p []byte) (n int, err error) {
	s := in.state
	if s != sOpen {
		return 0, fmt.Errorf("Read in state %s", s)
	}
	n, err = in.r.Read(p)
	if err != nil && err != io.EOF {
		in.Cancel()
	}
	return
}

type output struct {
	stream
	w *writer
}

func (out *output) Open(w *writer) error {
	if err := out.openCC(w); err != nil {
		return err
	}
	out.w = w
	return nil
}

func (out *output) Write(p []byte) (n int, err error) {
	s := out.state
	if s != sOpen {
		return 0, fmt.Errorf("Read in state %s", s)
	}
	n, err = out.w.Write(p)
	if err != nil {
		out.Cancel()
	}
	return
}

func signalHandler(cs ...canceler) chan<- struct{} {
	quit := make(chan struct{})
	sigch := make(chan os.Signal, 1)
	signal.Notify(sigch, os.Interrupt)
	go func() {
		select {
		case <-quit:
			signal.Stop(sigch)
			return
		case <-sigch:
			for _, cc := range cs {
				cc.Cancel()
			}
			os.Exit(7)
		}
	}()
	return quit
}

func processFile(path string, opts *options) {
	var err error
	var (
		in  input
		out output
	)
	defer func() {
		if err != nil {
			if err = out.Cancel(); err != nil {
				xlog.Warnf("cancel output error %s", err)
			}
			if err = in.Cancel(); err != nil {
				xlog.Warnf("cancel input error %s", err)
			}
		}
		if err = out.Close(); err != nil {
			xlog.Warnf("close output error %s", err)
		}
		if err = in.Close(); err != nil {
			xlog.Warnf("close input error %s", err)
		}
	}()

	quit := signalHandler(&in, &out)
	defer close(quit)

	r, err := newReaderPathOpts(path, opts)
	if err != nil {
		xlog.Warnf("file %s error %s", path, err)
		return
	}
	if err = in.Open(r); err != nil {
		xlog.Warnf("in.Open error %s", err)
		return
	}
	xlog.Debugf("reader for file %s opened", path)

	w, err := newWriterPathOpts(path, opts)
	if err != nil {
		xlog.Warnf("file %s error %s", path, err)
		return
	}
	if err = out.Open(w); err != nil {
		xlog.Warnf("in.Open error %s", err)
		return
	}
	xlog.Debugf("writer for file %s opened", path)

	if _, err = io.Copy(&out, &in); err != nil {
		xlog.Warnf("copy error %s", err)
		return
	}
}
