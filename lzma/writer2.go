package lzma

import (
	"bytes"
	"context"
	"errors"
	"io"
	"runtime"

	"github.com/ulikunitz/lz"
)

// Writer2Config provides the configuration parameters for an LZMA2 writer.
type Writer2Config struct {
	// Configuration for the LZ compressor.
	LZCfg lz.Configurator
	// Properties for the LZMA algorithm.
	Properties Properties
	// ZeroProperties are true if Properties are indeed zero.
	ZeroProperties bool

	// WorkerBufSize provides the buffer size that is provided to the
	// workers. Note the Workers number must be larger than 1 for this
	// parameter to apply.
	WorkerBufSize int

	// Number of workers processing data.
	Workers int
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

	if cfg.WorkerBufSize <= 0 {
		return errors.New("lzma: WorkerBufSize must be >= 0")
	}
	if cfg.Workers < 0 {
		return errors.New("lzma: Worker must be larger than 0")
	}

	return nil
}

// ApplyDefaults replaces zero values with default values.
func (cfg *Writer2Config) ApplyDefaults() {
	if cfg.LZCfg == nil {
		cfg.LZCfg = &lz.Config{}
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

	if cfg.WorkerBufSize == 0 {
		cfg.WorkerBufSize = 8 << 20 // 8 MiB
	}
	if cfg.Workers == 0 {
		cfg.Workers = runtime.NumCPU()
	}
}

// Writer2 is an interface that can Write, Close and Flush.
type Writer2 interface {
	io.WriteCloser
	Flush() error
}

// NewWriter2 generates an LZMA2 writer for the default configuration.
func NewWriter2(z io.Writer) (w Writer2, err error) {
	return NewWriter2Config(z, Writer2Config{})
}

// NewWriter2Config constrcuts an LZMA2 writer for a specific configuration.
// Note taht the implementation for cfg.Workers > 2 uses go routines.
func NewWriter2Config(z io.Writer, cfg Writer2Config) (w Writer2, err error) {
	cfg.ApplyDefaults()
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

type writer2Task struct {
	data  []byte
	zCh   chan []byte
	flush chan struct{}
}

type multiWriter struct {
	buf        []byte
	ctx        context.Context
	cancel     context.CancelFunc
	z          io.Writer
	compressCh chan writer2Task
	outCh      chan writer2Task
	errCh      chan error
	worker     int
	cfg        Writer2Config
	err        error
}

func (mw *multiWriter) Write(p []byte) (n int, err error) {
	panic("TODO")
}

func (mw *multiWriter) Flush() error {
	panic("TODO")
}

func (mw *multiWriter) Close() error {
	if mw.err != nil {
		return mw.err
	}
	defer mw.cancel()
	var err error
	if err = mw.Flush(); err != nil {
		mw.err = err
		return err
	}
	var zero [1]byte
	if _, err = mw.z.Write(zero[:]); err != nil {
		mw.err = err
		return err
	}
	return nil
}

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

func outputCompressedData(ctx context.Context, z io.Writer, outCh chan writer2Task, errCh chan error) {
	for {
		select {
		case <-ctx.Done():
			return
		case tsk := <-outCh:
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
				if tsk.flush != nil {
					select {
					case <-ctx.Done():
						return
					case tsk.flush <- struct{}{}:
						break
					}
				}
			}
		}
	}
}
