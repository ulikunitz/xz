package lzma

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"runtime"
	"testing"

	"github.com/ulikunitz/lz"
)

// Writer2Config provides the configuration parameters for an LZMA2 writer.
type Writer2Config struct {
	// DictSize sets the dictionary size.
	DictSize int

	// Properties for the LZMA algorithm.
	Properties Properties
	// ZeroProperties indicate that the Properties is indeed zero
	ZeroProperties bool

	// Number of workers processing data.
	Workers int
	// Size of buffer used by the worker.
	WorkerBufferSize int

	// Configuration for the LZ compressor.
	LZ lz.SeqConfig
}

// Verify checks whether the configuration is consistent and correct. Usually
// call SetDefaults before this method.
func (cfg *Writer2Config) Verify() error {
	var err error
	if cfg == nil {
		return errors.New("lzma: Writer2Config pointer must not be nil")
	}

	if cfg.LZ == nil {
		return errors.New("lzma: Writer2Config field LZCfg is nil")
	}

	if err = cfg.LZ.Verify(); err != nil {
		return err
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
		bc := cfg.LZ.BufConfig()
		if cfg.WorkerBufferSize > bc.BufferSize {
			return errors.New(
				"lzma: sequence buffer size must be" +
					" less or equal than worker buffer size")
		}
	}

	return nil
}

// fixBufConfig computes the sequence buffer configuration in a way that works
// for lzma. ShrinkSize cannot be smaller than the window size or the size of an
// uncompressed chunk.
func fixBufConfig(cfg lz.SeqConfig, windowSize int) {
	bc := cfg.BufConfig()
	bc.WindowSize = windowSize
	bc.ShrinkSize = bc.WindowSize
	bc.BufferSize = 2 * bc.WindowSize

	const minBufferSize = 256 << 10
	if bc.BufferSize < minBufferSize {
		bc.BufferSize = minBufferSize
	}

	// We need shrink size at least as large as an uncompressed chunk can
	// be. Otherwise we may not be able to copy the data into the chunk.
	const minShrinkSize = 1 << 16
	if bc.ShrinkSize < minShrinkSize {
		bc.ShrinkSize = minShrinkSize
	}
	cfg.SetBufConfig(bc)
}

// SetDefaults replaces zero values with default values. The workers variable
// will be set to the number of CPUs.
func (cfg *Writer2Config) SetDefaults() {
	if cfg.LZ == nil {
		dhsCfg := &lz.DHSConfig{WindowSize: cfg.DictSize}
		cfg.LZ = dhsCfg

	} else if cfg.DictSize > 0 {
		bc := cfg.LZ.BufConfig()
		bc.WindowSize = cfg.DictSize
		cfg.LZ.SetBufConfig(bc)
	}
	cfg.LZ.SetDefaults()
	bc := cfg.LZ.BufConfig()
	fixBufConfig(cfg.LZ, bc.WindowSize)

	var zeroProps = Properties{}
	if cfg.Properties == zeroProps && !cfg.ZeroProperties {
		cfg.Properties = Properties{3, 0, 2}
	}

	if cfg.Workers == 0 {
		cfg.Workers = runtime.GOMAXPROCS(0)
	}

	if cfg.WorkerBufferSize == 0 && cfg.Workers > 1 {
		cfg.WorkerBufferSize = 1 << 20
		bc := cfg.LZ.BufConfig()
		if cfg.WorkerBufferSize > bc.BufferSize {
			bc.BufferSize = cfg.WorkerBufferSize
			cfg.LZ.SetBufConfig(bc)
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
	cfg.SetDefaults()
	bc := cfg.LZ.BufConfig()
	if cfg.Workers > 1 && cfg.WorkerBufferSize > bc.BufferSize {
		bc.BufferSize = cfg.WorkerBufferSize
		cfg.LZ.SetBufConfig(bc)
	}
	if err = cfg.Verify(); err != nil {
		return nil, err
	}

	if cfg.Workers == 1 {
		seq, err := cfg.LZ.NewSequencer()
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
	mw := &mtWriter{
		// extra margin is an optimization for the sequencers
		buf:    make([]byte, 0, cfg.WorkerBufferSize+7),
		ctx:    ctx,
		cancel: cancel,
		taskCh: make(chan mtwTask, cfg.Workers),
		outCh:  make(chan mtwOutput, cfg.Workers),
		errCh:  make(chan error, 1),
		z:      z,
		cfg:    cfg,
	}

	go mtwWriteOutput(mw.ctx, mw.outCh, mw.z, mw.errCh)

	return mw, nil
}

type mtWriter struct {
	buf     []byte
	ctx     context.Context
	cancel  context.CancelFunc
	taskCh  chan mtwTask
	outCh   chan mtwOutput
	errCh   chan error
	z       io.Writer
	workers int
	cfg     Writer2Config
	err     error
}

func (w *mtWriter) DictSize() int {
	return w.cfg.LZ.BufConfig().WindowSize
}

func (w *mtWriter) Write(p []byte) (n int, err error) {
	if w.err != nil {
		return 0, w.err
	}
	select {
	case err = <-w.errCh:
		w.err = err
		w.cancel()
		return n, err
	default:
	}
	for len(p) > 0 {
		k := w.cfg.WorkerBufferSize - len(w.buf)
		if k >= len(p) {
			w.buf = append(w.buf, p...)
			n += len(p)
			return n, nil
		}
		if w.workers < w.cfg.Workers {
			go mtwWork(w.ctx, w.taskCh, w.cfg)
			w.workers++
		}
		w.buf = append(w.buf, p[:k]...)
		zCh := make(chan []byte, 1)
		select {
		case err = <-w.errCh:
			w.err = err
			w.cancel()
			return n, err
		case w.taskCh <- mtwTask{data: w.buf, zCh: zCh}:
		}
		select {
		case err = <-w.errCh:
			w.err = err
			w.cancel()
			return n, err
		case w.outCh <- mtwOutput{zCh: zCh}:
		}
		// extra margin is an optimization for the sequence buffers
		w.buf = make([]byte, 0, w.cfg.WorkerBufferSize+7)
		n += k
		p = p[k:]
	}
	return n, nil
}

func (w *mtWriter) Flush() error {
	if w.err != nil {
		return w.err
	}
	var err error
	select {
	case err = <-w.errCh:
		w.err = err
		w.cancel()
		return err
	default:
	}
	if w.workers < w.cfg.Workers {
		go mtwWork(w.ctx, w.taskCh, w.cfg)
		w.workers++
	}
	flushCh := make(chan struct{}, 1)
	var zCh chan []byte
	if len(w.buf) > 0 {
		zCh = make(chan []byte, 1)
		select {
		case err = <-w.errCh:
			w.err = err
			w.cancel()
			return err
		case w.taskCh <- mtwTask{data: w.buf, zCh: zCh}:
		}
		// extra margin is an optimization for the sequencers
		w.buf = make([]byte, 0, w.cfg.WorkerBufferSize+7)
	}
	select {
	case err = <-w.errCh:
		w.err = err
		w.cancel()
		return err
	case w.outCh <- mtwOutput{flushCh: flushCh, zCh: zCh}:
	}
	select {
	case err = <-w.errCh:
		w.err = err
		w.cancel()
		return err
	case <-flushCh:
	}
	return nil
}

var zero = make([]byte, 1)

func (w *mtWriter) Close() error {
	if w.err != nil {
		return w.err
	}
	defer w.cancel()
	var err error
	if err = w.Flush(); err != nil {
		w.err = err
		return err
	}
	if _, err = w.z.Write(zero); err != nil {
		w.err = err
		return err
	}
	w.err = errClosed
	return nil
}

type mtwOutput struct {
	flushCh chan<- struct{}
	zCh     <-chan []byte
}

type mtwTask struct {
	data []byte
	zCh  chan<- []byte
}

func mtwWriteOutput(ctx context.Context, outCh <-chan mtwOutput, z io.Writer, errCh chan<- error) {
	var (
		o    mtwOutput
		data []byte
	)
	for {
		select {
		case <-ctx.Done():
			return
		case o = <-outCh:
		}
		if o.zCh != nil {
			select {
			case <-ctx.Done():
				return
			case data = <-o.zCh:
			}
			if _, err := z.Write(data); err != nil {
				select {
				case <-ctx.Done():
					return
				case errCh <- err:
					return
				}
			}
		}
		if o.flushCh != nil {
			select {
			case <-ctx.Done():
				return
			case o.flushCh <- struct{}{}:
			}
		}
	}
}

func mtwWork(ctx context.Context, taskCh <-chan mtwTask, cfg Writer2Config) {
	seq, err := cfg.LZ.NewSequencer()
	if err != nil {
		panic(fmt.Errorf("NewSequencer error %s", err))
	}
	var (
		tsk mtwTask
		w   chunkWriter
	)
	for {
		select {
		case <-ctx.Done():
			return
		case tsk = <-taskCh:
		}
		buf := new(bytes.Buffer)
		if err := w.init(buf, seq, tsk.data, cfg.Properties); err != nil {
			panic(fmt.Errorf("w.init error %s", err))
		}
		if err := w.FlushContext(ctx); err != nil {
			if errors.Is(err, context.Canceled) ||
				errors.Is(err, context.DeadlineExceeded) {
				return

			}
			panic(fmt.Errorf("w.FlushContext error %s", err))
		}
		select {
		case <-ctx.Done():
			return
		case tsk.zCh <- buf.Bytes():
		}
	}
}

func TestWriter2ConfigDictSize(t *testing.T) {
	cfg := Writer2Config{DictSize: 4096}
	cfg.SetDefaults()
	if err := cfg.Verify(); err != nil {
		t.Fatalf("DictSize set without lzCfg: %s", err)
	}

	lzCfg := &lz.DHSConfig{WindowSize: 4097}
	cfg = Writer2Config{
		LZ:       lzCfg,
		DictSize: 4098,
	}
	cfg.SetDefaults()
	bc := cfg.LZ.BufConfig()
	if bc.WindowSize != 4098 {
		t.Fatalf("sbCfg.windowSize %d; want %d", bc.WindowSize, 4098)
	}
}
