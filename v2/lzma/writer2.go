// SPDX-FileCopyrightText: © 2014 Ulrich Kunitz
//
// SPDX-License-Identifier: BSD-3-Clause

package lzma

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"runtime"

	"github.com/ulikunitz/lz"
)

// Writer2Config provides the configuration parameters for an LZMA2 writer.
type Writer2Config struct {
	// WindowSize sets the dictionary size.
	WindowSize int
	// BufferSize sets the size of the buffer used by the LZ parser. It
	// defines the work size for parallel compression.
	BufferSize int

	// Properties for the LZMA algorithm.
	Properties *Properties

	// Number of workers processing data.
	Workers int

	// PathFinder describes the mechanism to select a match at a given
	// position of the uncompressed data. The default is "greedy".
	PathFinder string

	// Mapper is the name of the mapper to use for the LZ parser. The
	// default is "hash_2:16".
	Mapper string
}

type writer2JSONConfig struct {
	Format     string
	WindowSize int         `json:",omitzero"`
	BufferSize int         `json:",omitzero"`
	Properties *Properties `json:",omitzero"`
	Workers    int         `json:",omitzero"`
	PathFinder string      `json:",omitzero"`
	Mapper     string      `json:",omitzero"`
}

// MarshalJSON marshals the writer configuration into JSON.
func (cfg *Writer2Config) MarshalJSON() ([]byte, error) {
	c := writer2JSONConfig{
		Format:     "lzma2",
		WindowSize: cfg.WindowSize,
		BufferSize: cfg.BufferSize,
		Properties: cfg.Properties,
		Workers:    cfg.Workers,
		PathFinder: cfg.PathFinder,
		Mapper:     cfg.Mapper,
	}
	return json.Marshal(&c)
}

func (cfg *Writer2Config) UnmarshalJSON(data []byte) error {
	var c writer2JSONConfig
	if err := json.Unmarshal(data, &c); err != nil {
		return err
	}
	if c.Format != "lzma2" {
		return fmt.Errorf("lzma: invalid format %q", c.Format)
	}
	*cfg = Writer2Config{
		WindowSize: c.WindowSize,
		BufferSize: c.BufferSize,
		Workers:    c.Workers,
		PathFinder: c.PathFinder,
		Mapper:     c.Mapper,
	}
	if c.Properties != nil {
		cfg.Properties = c.Properties
	}
	return nil
}

// verify checks whether the configuration is consistent and correct. Normally,
// setDefaults should be called before this method.
func (cfg *Writer2Config) verify() error {
	var err error
	if cfg == nil {
		return errors.New("lzma: Writer2Config pointer must not be nil")
	}

	if cfg.Workers < 1 {
		return errors.New("lzma: Worker must be larger than 0")
	}

	if cfg.WindowSize <= 0 {
		return errors.New("lzma: WindowSize must be larger than 0")
	}

	if cfg.Workers == 1 {
		if !(cfg.WindowSize < cfg.BufferSize) {
			return errors.New(
				"lzma: BufferSize must be larger than WindowSize")
		}
	} else {
		if !(cfg.WindowSize <= cfg.BufferSize) {
			return errors.New(
				"lzma: BufferSize must be larger or equal than WindowSize")
		}
	}

	if cfg.Properties == nil {
		return errors.New("lzma: Properties must be set")
	}

	p := cfg.Properties
	if err = p.verify(); err != nil {
		return err
	}
	if p.LC+p.LP > 4 {
		return errors.New(
			"lzma: LC + LP must be smaller or equal than 4")
	}

	return nil
}

// SetDefaults replaces zero values with default values. The workers variable
// will be set to the number of CPUs.
func (cfg *Writer2Config) SetDefaults() {
	if cfg.Workers == 0 {
		cfg.Workers = runtime.GOMAXPROCS(0)
	}

	if cfg.Properties == nil {
		cfg.Properties = &Properties{3, 0, 2}
	}

	if cfg.Workers == 1 {
		if cfg.WindowSize == 0 {
			if cfg.BufferSize > 0 {
				cfg.WindowSize = cfg.BufferSize / 2
			} else {
				cfg.WindowSize = 8 << 20
			}
		}
		if cfg.BufferSize == 0 {
			cfg.BufferSize = 2 * cfg.WindowSize
		}
		return
	}

	if cfg.WindowSize == 0 {
		if cfg.BufferSize > 0 {
			cfg.WindowSize = cfg.BufferSize
		} else {
			cfg.WindowSize = 8 << 20
		}
	}
	if cfg.BufferSize == 0 {
		cfg.BufferSize = cfg.WindowSize
	}
}

// clone returns a deep copy of the writer configuration.
func (cfg Writer2Config) clone() Writer2Config {
	c := cfg
	if cfg.Properties != nil {
		c.Properties = new(*cfg.Properties)
	}
	return c
}

// Writer2 is an interface that can Write, Close and Flush.
type Writer2 interface {
	io.WriteCloser
	Flush() error
	WindowSize() int
}

// NewWriter2 generates an LZMA2 writer for the default configuration.
func NewWriter2(z io.Writer) (w Writer2, err error) {
	return NewWriter2Config(z, Writer2Config{})
}

// NewWriter2Config constructs an LZMA2 writer for a specific configuration.
// Note that the implementation for options.Workers > 1 uses go routines.
func NewWriter2Config(z io.Writer, cfg Writer2Config) (w Writer2, err error) {
	cfg = cfg.clone()
	cfg.SetDefaults()
	if err = cfg.verify(); err != nil {
		return nil, err
	}

	if cfg.Workers == 1 {
		lzCfg := lz.ParserConfig{
			WindowSize:    new(cfg.WindowSize),
			RetentionSize: new(cfg.WindowSize),
			BufferSize:    cfg.BufferSize,
			PathFinder:    cfg.PathFinder,
			Mapper:        cfg.Mapper,
			MinMatchLen:   2,
			MaxMatchLen:   273,
		}
		parser, err := lz.NewParser(lzCfg)
		if err != nil {
			return nil, err
		}
		var cw chunkWriter
		if err = cw.init(z, parser, nil, *cfg.Properties); err != nil {
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
	cfg        Writer2Config
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
			go mtwWork(w.ctx, w.taskCh, w.errCh, w.cfg)
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
		go mtwWork(w.ctx, w.taskCh, w.errCh, w.cfg)
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

func mtwWork(ctx context.Context, taskCh <-chan mtwTask, errCh chan<- error, cfg Writer2Config) {
	lzCfg := lz.ParserConfig{
		WindowSize:    new(cfg.WindowSize),
		RetentionSize: new(0),
		BufferSize:    cfg.BufferSize,
		PathFinder:    cfg.PathFinder,
		Mapper:        cfg.Mapper,
		MinMatchLen:   2,
		MaxMatchLen:   273,
	}

	parser, err := lz.NewParser(lzCfg)
	if err != nil {
		select {
		case <-ctx.Done():
		case errCh <- fmt.Errorf("lzma: NewParser error %w", err):
		}
		return
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
		if err := w.init(buf, parser, tsk.data, *cfg.Properties); err != nil {
			select {
			case <-ctx.Done():
			case errCh <- fmt.Errorf("w.init error %w", err):
			}
			return
		}
		if err := w.FlushContext(ctx); err != nil {
			if errors.Is(err, context.Canceled) ||
				errors.Is(err, context.DeadlineExceeded) {
				return
			}
			select {
			case <-ctx.Done():
			case errCh <- fmt.Errorf("w.FlushContext error %w", err):
			}
			return
		}
		select {
		case <-ctx.Done():
			return
		case tsk.zCh <- buf.Bytes():
		}
	}
}
