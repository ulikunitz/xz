package lzma

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
)

// Reader2Config provides the dictionary size parameter for a LZMA2 reader.
//
// Note that the parallel decoding will only work if the stream has been
// parallel encoded and the WorkerBufferSize is large enough. The reader stream
// is read until worker buffer size to check whether this is true if the Workers
// field is larger than 1.
type Reader2Config struct {
	// DictSize provides the maximum dictionary size supported.
	DictSize int
	// Workers gives the maximum number of decompressing workers.
	Workers int
	// WorkerBufferSize give the maximum size of uncompressed data that can be
	// decoded by a single worker.
	WorkerBufferSize int
}

// Verify checks the validity of dictionary size.
func (cfg *Reader2Config) Verify() error {
	if cfg.DictSize < minDictSize {
		return fmt.Errorf(
			"lzma: dictionary size must be larger or"+
				" equal %d bytes", minDictSize)
	}

	if cfg.Workers < 0 {
		return errors.New("lzma: Worker must be larger than 0")
	}

	if cfg.WorkerBufferSize <= 0 {
		return errors.New(
			"lzma: WorkerBufferSize must be greater than 0")
	}

	return nil
}

// ApplyDefaults sets a default value for the dictionary size.
func (cfg *Reader2Config) ApplyDefaults() {
	if cfg.DictSize == 0 {
		cfg.DictSize = 8 << 20
	}

	if cfg.Workers == 0 {
		cfg.Workers = 1
	}

	if cfg.WorkerBufferSize == 0 {
		cfg.WorkerBufferSize = 1 << 20
	}
}

// NewReader2 creates a LZMA2 reader. Note that the interface is a ReadCloser,
// so it has to be closed after usage.
func NewReader2(z io.Reader, dictSize int) (r io.ReadCloser, err error) {
	return NewReader2Config(z, Reader2Config{DictSize: dictSize})
}

// NewReader2Config generates an LZMA2 reader using the configuration parameter
// attribute. Note that the code returns a ReadCloser, which has to be clsoed
// after reading.
func NewReader2Config(z io.Reader, cfg Reader2Config) (r io.ReadCloser, err error) {
	cfg.ApplyDefaults()
	if err = cfg.Verify(); err != nil {
		return nil, err
	}
	if cfg.Workers <= 1 {
		var cr chunkReader
		cr.init(z, cfg.DictSize)
		return io.NopCloser(&cr), nil
	}
	return newMTReader(cfg, z), nil
}

type mtReaderTask struct {
	// compressed stream consisting of chunks
	z io.Reader
	// uncompressed size;  less than zero if unknown (requires pipe)
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

func newMTReader(cfg Reader2Config, z io.Reader) *mtReader {
	ctx, cancel := context.WithCancel(context.Background())
	tskCh := make(chan mtReaderTask)
	outCh := make(chan mtReaderTask)
	go mtrGenerate(ctx, z, cfg.WorkerBufferSize, tskCh, outCh)
	for i := 0; i < cfg.Workers; i++ {
		go mtrWork(ctx, cfg.DictSize, tskCh)
	}
	return &mtReader{
		cancel: cancel,
		outCh:  outCh,
	}
}

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

func (r *mtReader) Close() error {
	if r.err == errClosed {
		return errClosed
	}
	r.cancel()
	r.err = errClosed
	return nil
}

func mtrGenerate(ctx context.Context, z io.Reader, bufSize int, tskCh, outCh chan<- mtReaderTask) {
	r := bufio.NewReader(z)
	for ctx.Err() == nil {
		buf := new(bytes.Buffer)
		buf.Grow(bufSize)
		tsk := mtReaderTask{
			rCh: make(chan io.Reader, 1),
		}
		size, parallel, err := splitStream(buf, r, bufSize)
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
			tsk.z = buf
			tsk.size = size
		} else {
			tsk.z = io.MultiReader(buf, r)
			tsk.size = -1
			err = io.EOF
		}
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
	}
}

type errReader struct{ err error }

func (r *errReader) Read(p []byte) (n int, err error) { return 0, r.err }

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
			chr.noEOS = false
			r, w := io.Pipe()
			select {
			case <-ctx.Done():
				return
			case tsk.rCh <- r:
			}
			if _, err := io.Copy(w, &chr); err != nil {
				if err = w.CloseWithError(err); err != nil {
					panic(fmt.Errorf(
						"w.CloseWithError error %s",
						err))
				}
			}
			if err := w.Close(); err != nil {
				panic(fmt.Errorf("w.Close() error %s", err))
			}
		}
	}
}

// splitStream splits the LZMA stream into blocks that can be processed in
// parallel. Such blocks need to start with a dictionary reset. If such a block
// cannot be found that is less or equal size then false is returned and the
// write contains a series of chunks and the last chunk headere. The number n
// contains the size of the decompressed block. If ok is false n will be zero.
func splitStream(w io.Writer, z *bufio.Reader, size int) (n int, ok bool, err error) {
	for {
		hdr, k, err := peekChunkHeader(z)
		if err != nil {
			if err == io.EOF {
				err = io.ErrUnexpectedEOF
			}
			return 0, false, err
		}
		switch hdr.control {
		case cUD, cCSPD:
			if n > 0 {
				return n, true, nil
			}
		case cEOS:
			return n, true, io.EOF
		}
		if hdr.control&(1<<7) == 0 {
			k += hdr.size
		} else {
			k += hdr.compressedSize
		}
		n += hdr.size
		if n > size {
			return 0, false, io.EOF
		}
		if _, err := io.CopyN(w, z, int64(k)); err != nil {
			if err == io.EOF {
				err = io.ErrUnexpectedEOF
			}
			return 0, false, err
		}
	}
}
