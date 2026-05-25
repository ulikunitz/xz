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

	"github.com/ulikunitz/lz"
)

// writer2Config provides the configuration parameters for an LZMA2 writer.
type writer2Config struct {
	// WindowSize sets the dictionary size.
	WindowSize int
	// BufferSize sets the size of the buffer used by the LZ parser. It
	// defines the work size for parallel compression.
	BufferSize int

	// Properties for the LZMA algorithm.
	Properties Properties

	// Number of workers processing data.
	Workers int

	PathFinder string
	Mapper     string

	MinMatchLen int
	MaxMatchLen int
}

type Writer2Option interface {
	updateWriter2Config(*writer2Config) error
}

type AllWriterOption interface {
	WriterOption
	Writer2Option
}

type RW2Option interface {
	Reader2Option
	Writer2Option
}

// verify checks whether the configuration is consistent and correct. Usually
// call SetDefaults before this method.
func (cfg *writer2Config) verify() error {
	var err error
	if cfg == nil {
		return errors.New("lzma: Writer2Options pointer must not be nil")
	}

	if cfg.WindowSize <= 0 {
		return errors.New("lzma: WindowSize must be larger than 0")
	}
	if !(cfg.WindowSize <= cfg.BufferSize) {
		return errors.New(
			"lzma: BufferSize must be larger or equal than WindowSize")
	}

	if err = cfg.Properties.verify(); err != nil {
		return err
	}

	if cfg.Workers < 1 {
		return errors.New("lzma: Worker must be larger than 0")
	}

	return nil
}

// Writer2 is an interface that can Write, Close and Flush.
type Writer2 interface {
	io.WriteCloser
	Flush() error
	WindowSize() int
}

// NewWriter2 generates an LZMA2 writer for the default configuration.
func NewWriter2(z io.Writer, options ...Writer2Option) (w Writer2, err error) {
	cfg := writer2Config{
		WindowSize:  -1,
		BufferSize:  0,
		Workers:     1,
		Properties:  Properties{3, 0, 2},
		PathFinder:  "greedy",
		Mapper:      "hash_3:16",
		MinMatchLen: minMatchLen,
		MaxMatchLen: maxMatchLen,
	}
	for _, option := range options {
		if err = option.updateWriter2Config(&cfg); err != nil {
			return nil, err
		}
	}

	if cfg.WindowSize < 0 {
		if cfg.BufferSize > 0 {
			switch cfg.Workers {
			case 1:
				cfg.WindowSize = cfg.BufferSize / 2
			default:
				cfg.WindowSize = cfg.BufferSize
			}
		} else {
			cfg.WindowSize = 1 << 20
		}
	}

	if cfg.BufferSize == 0 {
		switch cfg.Workers {
		case 1:
			cfg.BufferSize = max(2*cfg.WindowSize, 1024)
		default:
			cfg.BufferSize = max(cfg.WindowSize, 1024)
		}
	}

	if err = cfg.verify(); err != nil {
		return nil, err
	}

	if cfg.Workers == 1 {
		parser, err := lz.NewParser(
			lz.WithWindowSize(cfg.WindowSize),
			lz.WithRetentionSize(cfg.WindowSize),
			lz.WithBufferSize(cfg.BufferSize),
			lz.WithPathFinder(cfg.PathFinder),
			lz.WithMapper(cfg.Mapper),
			lz.WithMinMatchLen(cfg.MinMatchLen),
			lz.WithMaxMatchLen(cfg.MaxMatchLen),
		)
		if err != nil {
			return nil, err
		}
		var cw chunkWriter
		if err = cw.init(z, parser, nil, cfg.Properties); err != nil {
			return nil, err
		}
		return &cw, nil
	}

	ctx, cancel := context.WithCancel(context.Background())
	mw := &mtWriter{
		// extra margin is an optimization for the sequencers
		buf:        make([]byte, 0, cfg.BufferSize+7),
		ctx:        ctx,
		cancel:     cancel,
		taskCh:     make(chan mtwTask, cfg.Workers),
		outCh:      make(chan mtwOutput, cfg.Workers),
		errCh:      make(chan error, 1),
		z:          z,
		windowSize: cfg.WindowSize,
		cfg:        cfg,
	}

	go mtwWriteOutput(mw.ctx, mw.outCh, mw.z, mw.errCh)

	return mw, nil
}

type mtWriter struct {
	buf        []byte
	ctx        context.Context
	cancel     context.CancelFunc
	taskCh     chan mtwTask
	outCh      chan mtwOutput
	errCh      chan error
	z          io.Writer
	workers    int
	cfg        writer2Config
	err        error
	windowSize int
}

func (w *mtWriter) WindowSize() int {
	return w.windowSize
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
		k := w.cfg.BufferSize - len(w.buf)
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
		w.buf = make([]byte, 0, w.cfg.BufferSize+7)
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
		w.buf = make([]byte, 0, w.cfg.BufferSize+7)
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

func mtwWork(ctx context.Context, taskCh <-chan mtwTask, cfg writer2Config) {
	parser, err := lz.NewParser(
		lz.WithWindowSize(cfg.WindowSize),
		lz.WithRetentionSize(0),
		lz.WithBufferSize(cfg.WindowSize),
		lz.WithPathFinder(cfg.PathFinder),
		lz.WithMapper(cfg.Mapper),
		lz.WithMinMatchLen(cfg.MinMatchLen),
		lz.WithMaxMatchLen(cfg.MaxMatchLen),
	)
	if err != nil {
		panic(fmt.Errorf("xz: NewParser error %s", err))
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
		if err := w.init(buf, parser, tsk.data, cfg.Properties); err != nil {
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
