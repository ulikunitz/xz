package lzma

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"

	"github.com/ulikunitz/lz"
)

// Writer2Config provides the configuration parameters for an LZMA2 writer.
type Writer2Config struct {
	// Configuration for the LZ compressor.
	LZCfg lz.Configurator
	// Properties for the LZMA algorithm.
	Properties Properties
	// ZeroProperties indicate that the Properties is indeed zero
	ZeroProperties bool

	// Number of workers processing data.
	Workers int
	// Size of buffer used by the worker.
	WorkerBufferSize int
}

// Verify checks whether the configuration is consistent and correct. Usually
// call ApplyDefaults before this method.
func (cfg *Writer2Config) Verify() error {
	if cfg == nil {
		return errors.New("lzma: Writer2Config pointer must not be nil")
	}

	var err error
	type verifier interface {
		Verify() error
	}
	v, ok := cfg.LZCfg.(verifier)
	if ok {
		if err = v.Verify(); err != nil {
			return err
		}
	}

	if err = cfg.Properties.Verify(); err != nil {
		return err
	}

	if cfg.Workers < 0 {
		return errors.New("lzma: Worker must be larger than 0")
	}

	if cfg.Workers > 1 && cfg.WorkerBufferSize <= 0 {
		return errors.New(
			"lzma: WorkerBufferSize must be greater than 0")
	}

	if cfg.Workers > 1 {
		sbCfg := cfg.LZCfg.BufferConfig()
		if cfg.WorkerBufferSize > sbCfg.BufferSize {
			return errors.New(
				"lzma: sequence buffer size must be" +
					" less or equal than worker buffer size")
		}
	}

	return nil
}

// ApplyDefaults replaces zero values with default values. The workers variable
// will be set to the number of CPUs.
func (cfg *Writer2Config) ApplyDefaults() {
	if cfg.LZCfg == nil {
		var err error
		cfg.LZCfg, err = lz.Config(lz.Params{})
		if err != nil {
			panic(fmt.Errorf("lz.Config error %s", err))
		}
	}

	type ad interface {
		ApplyDefaults()
	}
	if a, ok := cfg.LZCfg.(ad); ok {
		a.ApplyDefaults()
	}

	var zeroProps = Properties{}
	if cfg.Properties == zeroProps && !cfg.ZeroProperties {
		cfg.Properties = Properties{3, 0, 2}
	}

	if cfg.Workers == 0 {
		// TODO: cfg.Workers = runtime.GOMAXPROCS(0)
		cfg.Workers = 1
	}

	if cfg.WorkerBufferSize == 0 && cfg.Workers > 1 {
		cfg.WorkerBufferSize = 1 << 20
		sbCfg := cfg.LZCfg.BufferConfig()
		if cfg.WorkerBufferSize > sbCfg.BufferSize {
			sbCfg.BufferSize = cfg.WorkerBufferSize
		}
	}
}

// Writer2 is an interface that can Write, Close and Flush.
type Writer2 interface {
	io.WriteCloser
	Flush() error
	DictSize() int
}

// NewWriter2 generates an LZMA2 writer for the default configuration.
func NewWriter2(z io.Writer) (w Writer2, err error) {
	return NewWriter2Config(z, Writer2Config{})
}

// NewWriter2Config constructs an LZMA2 writer for a specific configuration.
// Note that the implementation for cfg.Workers > 1 uses go routines.
func NewWriter2Config(z io.Writer, cfg Writer2Config) (w Writer2, err error) {
	cfg.ApplyDefaults()
	sbCfg := cfg.LZCfg.BufferConfig()
	if cfg.Workers > 1 && cfg.WorkerBufferSize > sbCfg.BufferSize {
		sbCfg.BufferSize = cfg.WorkerBufferSize
	}
	if err = cfg.Verify(); err != nil {
		return nil, err
	}

	if cfg.Workers == 1 {
		seq, err := cfg.LZCfg.NewSequencer()
		if err != nil {
			return nil, err
		}
		var cw chunkWriter
		if err = cw.init(z, seq, nil, cfg.Properties); err != nil {
			return nil, err
		}
		return &cw, nil
	}

	ctx, cancel := context.WithCancel(context.Background())
	mw := &multiWriter{
		ctx:        ctx,
		cancel:     cancel,
		z:          z,
		compressCh: make(chan writer2Task, cfg.Workers),
		outCh:      make(chan writer2Task, cfg.Workers),
		errCh:      make(chan error, 1),
		cfg:        cfg,
	}

	go outputCompressedData(mw.ctx, mw.z, mw.outCh, mw.errCh)

	return mw, nil
}

// writer2Task describes a task that is processed by the compressing worker and
// the output goroutine.
//
// The flushCh provides the information that the task and all tasks before it
// have been successfully processed.
type writer2Task struct {
	data    []byte
	zCh     chan []byte
	flushCh chan struct{}
}

// multiWriter describes the multithreaded writer that compresses data.
type multiWriter struct {
	buf        []byte
	ctx        context.Context
	cancel     context.CancelFunc
	z          io.Writer
	compressCh chan writer2Task
	outCh      chan writer2Task
	errCh      chan error
	workers    int
	cfg        Writer2Config
	err        error
}

// DictSize returns the dictionary size of the LZ sequencer.
func (mw *multiWriter) DictSize() int {
	seq, err := mw.cfg.LZCfg.NewSequencer()
	if err != nil {
		panic(err)
	}
	return seq.Buffer().WindowSize
}

// Write writes data into a buffer and if the buffer is large enough provides it
// to the compressing worker.
func (mw *multiWriter) Write(p []byte) (n int, err error) {
	if mw.err != nil {
		return 0, mw.err
	}
	select {
	case err = <-mw.errCh:
		mw.err = err
		mw.cancel()
		return 0, err
	default:
	}

	for len(p) > 0 {
		k := mw.cfg.WorkerBufferSize - len(mw.buf)
		if k > len(p) {
			mw.buf = append(mw.buf, p...)
			n += len(p)
			return n, nil
		}
		mw.buf = append(mw.buf, p[:k]...)
		if mw.workers < mw.cfg.Workers {
			seq, err := mw.cfg.LZCfg.NewSequencer()
			if err != nil {
				mw.err = err
				mw.cancel()
				return n, err
			}
			go compressWorker(mw.ctx, mw.compressCh, seq,
				mw.cfg.Properties)
		}
		task := writer2Task{
			data: mw.buf,
			zCh:  make(chan []byte, 1),
		}
		mw.buf = nil
		// TODO: remove
		if len(task.data) == 0 {
			panic("len(task.data) must be greater zero")
		}
		select {
		case err = <-mw.errCh:
			mw.err = err
			mw.cancel()
			return n, err
		case mw.compressCh <- task:
		}
		select {
		case err = <-mw.errCh:
			mw.err = err
			mw.cancel()
			return n, err
		case mw.outCh <- task:
		}
		n += k
		p = p[k:]
	}
	return n, nil
}

// Flush flushes all buffered data.
func (mw *multiWriter) Flush() error {
	if mw.err != nil {
		return mw.err
	}
	var err error
	select {
	case err = <-mw.errCh:
		mw.err = err
		mw.cancel()
		return err
	default:
	}
	if mw.workers < mw.cfg.Workers {
		seq, err := mw.cfg.LZCfg.NewSequencer()
		if err != nil {
			mw.err = err
			mw.cancel()
			return err
		}
		go compressWorker(mw.ctx, mw.compressCh, seq, mw.cfg.Properties)
	}
	flushCh := make(chan struct{}, 1)
	task := writer2Task{
		data:    mw.buf,
		zCh:     make(chan []byte, 1),
		flushCh: flushCh,
	}
	mw.buf = nil
	if len(task.data) > 0 {
		select {
		case err = <-mw.errCh:
			mw.err = err
			mw.cancel()
			return err
		case mw.compressCh <- task:
		}
	}
	select {
	case err = <-mw.errCh:
		mw.err = err
		mw.cancel()
		return err
	case mw.outCh <- task:
	}
	select {
	case err = <-mw.errCh:
		mw.err = err
		mw.cancel()
		return err
	case <-flushCh:
		return nil
	}
}

// Close closes the writer and terminates the LZMA2 stream.
func (mw *multiWriter) Close() error {
	if mw.err != nil {
		return mw.err
	}
	defer mw.cancel()
	var err error
	if err = mw.Flush(); err != nil {
		return err
	}
	var zero [1]byte
	if _, err = mw.z.Write(zero[:]); err != nil {
		mw.err = err
		return err
	}
	mw.err = errClosed
	return nil
}

// compressWorker compressed the data provided by the writer2Task and provides
// the compressed information to the zCh.
func compressWorker(ctx context.Context, compressCh chan writer2Task, seq lz.Sequencer, props Properties) {
	var (
		err error
		w   chunkWriter
	)
	for {
		select {
		case <-ctx.Done():
			return
		case tsk := <-compressCh:
			buf := new(bytes.Buffer)
			if err = w.init(buf, seq, tsk.data, props); err != nil {
				panic(err)
			}
			if err = w.FlushContext(ctx); err != nil {
				if errors.Is(err, context.Canceled) ||
					errors.Is(err,
						context.DeadlineExceeded) {
					return
				}
				panic(err)
			}
			select {
			case tsk.zCh <- buf.Bytes():
				break
			case <-ctx.Done():
				return
			}
		}
	}
}

// outputCompressedData provides the single output channel that writes the data
// in the correct sequence to the underlying writer.
func outputCompressedData(ctx context.Context, z io.Writer, outCh chan writer2Task, errCh chan error) {
	for {
		select {
		case <-ctx.Done():
			return
		case tsk := <-outCh:
			if len(tsk.data) > 0 {
				select {
				case <-ctx.Done():
					return
				case data := <-tsk.zCh:
					if _, err := z.Write(data); err != nil {
						select {
						case <-ctx.Done():
							return
						case errCh <- err:
							return
						}
					}
				}
			}
			if tsk.flushCh != nil {
				select {
				case <-ctx.Done():
					return
				case tsk.flushCh <- struct{}{}:
					break
				}
			}
		}
	}
}
