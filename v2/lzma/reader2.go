// SPDX-FileCopyrightText: © 2014 Ulrich Kunitz
//
// SPDX-License-Identifier: BSD-3-Clause

package lzma

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
)

// reader2Config provides the dictionary size parameter for a LZMA2 reader.
//
// Note that the parallel decoding will only work if the stream has been encoded
// with multiple workers and the WorkerBufferSize is large enough. If the worker
// buffer size is too small no worker thread will be used for decompression.
type reader2Config struct {
	// WindowSize provides the maximum dictionary size supported.
	WindowSize int
	// BufferSize give the size of the decoder buffer and if there are more
	// workers than 1, gives the work size for each worker.
	BufferSize int
	// Workers gives the maximum number of decompressing workers.
	Workers int
}

// verify checks the validity of dictionary size.
func (cfg *reader2Config) verify() error {
	if cfg.WindowSize < minDictSize {
		return fmt.Errorf(
			"lzma: dictionary size must be larger or"+
				" equal %d bytes", minDictSize)
	}

	if cfg.Workers <= 0 {
		return errors.New("lzma: Worker must be larger than 0")
	}

	if !(cfg.WindowSize <= cfg.BufferSize) {
		return fmt.Errorf(
			"lzma: BufferSize=%d must be greater than"+
				" or equal WindowSize=%d for single-threaded"+
				" reader",
			cfg.BufferSize, cfg.WindowSize)
	}
	return nil
}

// Reader2Option provides the interface for options that can be used to
// configure a LZMA2 reader. Note that the options have to be passed to
// NewReader2Options.
type Reader2Option interface {
	updateReader2Config(*reader2Config) error
}

type workersOption int

func (o workersOption) updateReader2Config(cfg *reader2Config) error {
	if o < 1 {
		return fmt.Errorf("lzma: number of workers must be > 0")
	}
	cfg.Workers = int(o)
	return nil
}

func (o workersOption) updateWriter2Config(cfg *writer2Config) error {
	if o < 1 {
		return fmt.Errorf("lzma: number of workers must be > 0")
	}
	cfg.Workers = int(o)
	return nil
}

// WithWorkers sets the number of workers for a LZMA2 reader. Note that the
// number of workers must be larger than 0 and that the stream has to be encoded
// with multiple workers and the WorkerBufferSize has to be large enough for the
// parallel decoding to work. If the worker buffer size is too small no worker
// thread will be used for decompression.
func WithWorkers(workers int) RW2Option {
	return workersOption(workers)
}

type bufferSizeOption int

func updateBufferSize[T any](cfg *T, bufferSize int) error {
	if bufferSize < 1 {
		return fmt.Errorf("lzma: buffer size must be > 0")
	}
	switch c := any(cfg).(type) {
	case *reader2Config:
		c.BufferSize = bufferSize
	case *writer2Config:
		c.BufferSize = bufferSize
	default:
		panic(fmt.Sprintf("unexpected type %T", cfg))
	}
	return nil
}

func (o bufferSizeOption) updateReader2Config(cfg *reader2Config) error {
	return updateBufferSize(cfg, int(o))
}

func (o bufferSizeOption) updateWriter2Config(cfg *writer2Config) error {
	return updateBufferSize(cfg, int(o))
}

func WithBufferSize(bufferSize int) RW2Option {
	return bufferSizeOption(bufferSize)
}

// NewReader2 creates a LZMA2 reader. Note that the interface is a ReadCloser,
// so it has to be closed after usage.
func NewReader2(z io.Reader, windowSize int, options ...Reader2Option) (r io.ReadCloser, err error) {
	if windowSize < minDictSize {
		return nil, fmt.Errorf(
			"lzma: dictionary size must be larger or"+
				" equal %d bytes", minDictSize)
	}
	cfg := reader2Config{
		WindowSize: windowSize,
		BufferSize: 2 * windowSize,
		Workers:    1,
	}
	for _, opt := range options {
		if err = opt.updateReader2Config(&cfg); err != nil {
			return nil, err
		}
	}

	if err = cfg.verify(); err != nil {
		return nil, err
	}
	if cfg.Workers <= 1 {
		var cr chunkReader
		cr.init(z, cfg.WindowSize)
		return io.NopCloser(&cr), nil
	}
	cfg.BufferSize = cfg.WindowSize
	return newMTReader(&cfg, z), nil
}

// mtReaderTask describes a single decompression task.
type mtReaderTask struct {
	// compressed stream consisting of chunks
	z io.Reader
	// uncompressed size; less than zero if unknown.
	size int
	// reader for decompressed data
	rCh chan io.Reader
}

// mtReader provides a multithreaded reader for LZMA2 streams.
type mtReader struct {
	cancel context.CancelFunc
	outCh  <-chan mtReaderTask
	err    error
	r      io.Reader
}

// newMTReader creates a new multithreading reader. Note that Close must be
// called to clean up.
func newMTReader(cfg *reader2Config, z io.Reader) *mtReader {
	ctx, cancel := context.WithCancel(context.Background())
	tskCh := make(chan mtReaderTask, cfg.Workers)
	outCh := make(chan mtReaderTask, cfg.Workers)
	go mtrGenerate(ctx, z, *cfg, tskCh, outCh)
	return &mtReader{
		cancel: cancel,
		outCh:  outCh,
	}
}

// Read reads the data from the multithreaded reader.
func (r *mtReader) Read(p []byte) (n int, err error) {
	if r.err != nil {
		return 0, r.err
	}
	for n < len(p) {
		if r.r == nil {
			tsk, ok := <-r.outCh
			if !ok {
				r.err = io.EOF
				if n == 0 {
					r.cancel()
					return 0, io.EOF
				}
				return n, nil
			}
			r.r = <-tsk.rCh
		}
		k, err := r.r.Read(p[n:])
		n += k
		if err != nil {
			if err == io.EOF {
				r.r = nil
				continue
			}
			r.err = err
			return n, err
		}
	}
	return n, nil
}

// Close closes the multithreading reader and stops all workers.
func (r *mtReader) Close() error {
	if r.err == errClosed {
		return errClosed
	}
	r.cancel()
	r.err = errClosed
	return nil
}

// mtrGenerate generates the tasks for the multithreaded reader. It should be
// started as go routine.
func mtrGenerate(ctx context.Context, z io.Reader, cfg reader2Config, tskCh, outCh chan mtReaderTask) {
	r := &hdrReader{r: z}
	workers := 0
	for ctx.Err() == nil {
		buf := new(bytes.Buffer)
		buf.Grow(cfg.BufferSize)
		tsk := mtReaderTask{
			rCh: make(chan io.Reader, 1),
		}
		size, parallel, err := splitStream(buf, r, cfg.BufferSize)
		if err != nil && err != io.EOF {
			tsk.rCh <- &errReader{err: err}
			select {
			case <-ctx.Done():
				return
			case outCh <- tsk:
			}
			close(outCh)
			return
		}
		if parallel {
			if workers < cfg.Workers {
				go mtrWork(ctx, cfg.WindowSize, tskCh)
				workers++
			}
			tsk.z = buf
			tsk.size = size
			select {
			case <-ctx.Done():
				return
			case tskCh <- tsk:
			}
			select {
			case <-ctx.Done():
				return
			case outCh <- tsk:
			}
			if err == io.EOF {
				close(outCh)
				return
			}
		} else {
			tsk.z = io.MultiReader(buf, r)
			tsk.size = -1
			chr := new(chunkReader)
			chr.init(tsk.z, cfg.WindowSize)
			chr.noEOS = false
			tsk.rCh <- chr
			select {
			case <-ctx.Done():
				return
			case outCh <- tsk:
			}
			close(outCh)
			return
		}
	}
}

// errReader is a reader that returns only an error.
type errReader struct{ err error }

// Read returns the error of the errReader.
func (r *errReader) Read(p []byte) (n int, err error) { return 0, r.err }

// mtrWork is the go routine function that does the actual decompression.
func mtrWork(ctx context.Context, dictSize int, tskCh <-chan mtReaderTask) {
	var chr chunkReader
	chr.init(nil, dictSize)
	for {
		var tsk mtReaderTask
		select {
		case <-ctx.Done():
			return
		case tsk = <-tskCh:
		}
		chr.reset(tsk.z)
		if tsk.size >= 0 {
			chr.noEOS = true
			buf := new(bytes.Buffer)
			buf.Grow(int(tsk.size))
			var r io.Reader
			if _, err := io.Copy(buf, &chr); err != nil {
				r = &errReader{err: err}
			} else {
				r = buf
			}
			select {
			case <-ctx.Done():
				return
			case tsk.rCh <- r:
			}
		} else {
			panic(fmt.Errorf("negative size not expected"))
		}
	}
}

type hdrReader struct {
	err error
	hdr []byte
	r   io.Reader
}

func (hr *hdrReader) Peek(p []byte) (n int, err error) {
	if hr.err != nil {
		return 0, hr.err
	}
	if len(hr.hdr) > 0 {
		n = copy(p, hr.hdr)
		if n >= len(p) {
			return n, nil
		}
		p = p[n:]
	}
	k, err := io.ReadFull(hr.r, p)
	n += int(k)
	hr.hdr = append(hr.hdr, p[:k]...)
	if err != nil {
		hr.err = err
		if n > 0 && err == io.EOF {
			err = nil
		}
	}
	return n, err
}

func (hr *hdrReader) Read(p []byte) (n int, err error) {
	if hr.err != nil {
		return 0, hr.err
	}
	if len(hr.hdr) > 0 {
		k := copy(p, hr.hdr)
		n += k
		k = copy(hr.hdr, hr.hdr[k:])
		hr.hdr = hr.hdr[:k]
		if k > 0 {
			return n, nil
		}
	}
	k, err := hr.r.Read(p[n:])
	n += k
	if err != nil {
		hr.err = err
		if n > 0 && err == io.EOF {
			err = nil
		}
	}
	return n, err
}

// splitStream splits the LZMA stream into blocks that can be processed in
// parallel. Such blocks need to start with a dictionary reset. If such a block
// cannot be found that is less or equal size then false is returned and the
// write contains a series of chunks and the last chunk header. The number n
// contains the size of the decompressed block. If ok is false n will be zero.
func splitStream(w io.Writer, hr *hdrReader, size int) (n int, ok bool, err error) {
	for {
		hdr, k, err := peekChunkHeader(hr)
		if err != nil {
			if err == io.EOF {
				err = io.ErrUnexpectedEOF
			}
			return n, false, err
		}
		switch hdr.Control {
		case CUD, CCSPD:
			if n > 0 {
				return n, true, nil
			}
		case CEOS:
			return n, true, io.EOF
		}
		if hdr.Control&(1<<7) == 0 {
			k += hdr.Size
		} else {
			k += hdr.CompressedSize
		}
		n += hdr.Size
		if n > size {
			return 0, false, io.EOF
		}
		if _, err := io.CopyN(w, hr, int64(k)); err != nil {
			if err == io.EOF {
				err = io.ErrUnexpectedEOF
			}
			return 0, false, err
		}
	}
}
