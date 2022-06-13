package lzma

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"runtime"
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
	// WorkerBufferSize give the maximum size of compressed data that can be
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
		cfg.Workers = runtime.NumCPU()
	}

	if cfg.WorkerBufferSize == 0 {
		cfg.WorkerBufferSize = 1 << 20
	}
}

// NewReader2 creates a LZMA2 reader.
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
	var cr chunkReader
	cr.init(z, cfg.DictSize)
	return io.NopCloser(&cr), nil
}

// TODO: change design

type mrTask struct {
	data  []byte
	outCh chan []byte
}

type multiReader struct {
	cancel context.CancelFunc
	outCh  chan mrTask
	errCh  chan error
	buf    bytes.Buffer
	err    error
}

func (mr *multiReader) init(z io.Reader, cfg Reader2Config) {
	*mr = multiReader{
		outCh: make(chan mrTask, cfg.Workers),
		errCh: make(chan error, 1),
	}

	var ctx context.Context
	ctx, mr.cancel = context.WithCancel(context.Background())
	go readChunks(ctx, z, mr.outCh, mr.errCh, cfg)
}

func readChunks(ctx context.Context, z io.Reader, outCh chan<- mrTask,
	err chan<- error, cfg Reader2Config) {

	taskCh := make(chan mrTask, cfg.Workers)

	_ = taskCh
	panic("TODO")
}

func processTasks(ctx context.Context, taskCh <-chan mrTask, errCh chan<- error,
	dictSize int) {
	var cr chunkReader
	if err := cr.init(nil, dictSize); err != nil {
		select {
		case errCh <- err:
		default:
		}
		return
	}
	for {
		select {
		case <-ctx.Done():
			return
		case tsk := <-taskCh:
			var buf bytes.Buffer
			cr.reset(bytes.NewReader(tsk.data))
			_, err := io.Copy(&buf, &cr)
			if err != nil {
				select {
				case errCh <- err:
				default:
				}
				return
			}
			select {
			case <-ctx.Done():
				return
			case tsk.outCh <- buf.Bytes():
			}
		}
	}
}

func (mr *multiReader) Close() error {
	if mr.err != nil {
		if mr.err == io.EOF {
			mr.err = errClosed
			return nil
		}
		return mr.err
	}
	mr.err = errClosed
	mr.cancel()
	return nil
}

func (mr *multiReader) Read(p []byte) (n int, err error) {
	if mr.err != nil {
		return 0, mr.err
	}
	for {
		select {
		case mr.err = <-mr.errCh:
			mr.cancel()
			return n, mr.err
		default:
			k, _ := mr.buf.Read(p[n:])
			n += k
			if n == len(p) {
				return n, nil
			}
		}
		select {
		case mr.err = <-mr.errCh:
			mr.cancel()
			return n, mr.err
		case tsk, ok := <-mr.outCh:
			if !ok {
				mr.err = io.EOF
				mr.cancel()
				if n > 0 {
					return n, nil
				}
				return 0, io.EOF
			}
			select {
			case mr.err = <-mr.errCh:
				mr.cancel()
				return n, mr.err
			case data := <-tsk.outCh:
				mr.buf.Write(data)
			}
		}
	}
}
