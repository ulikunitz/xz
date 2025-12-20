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

// Writer2Options provides the configuration parameters for an LZMA2 writer.
type Writer2Options struct {
	// WindowSize sets the dictionary size.
	WindowSize int

	// Properties for the LZMA algorithm.
	Properties Properties
	// FixedProperties indicate that the Properties is indeed zero
	FixedProperties bool

	// Number of workers processing data.
	Workers int
	// Size of buffer used by the worker.
	WorkSize int

	// Options for the LZ parser.
	ParserOptions lz.Configurator
}

// UnmarshalJSON parses the JSON representation for Writer2Options and sets the
// cfg value accordingly.
func (cfg *Writer2Options) UnmarshalJSON(p []byte) error {
	var err error
	s := struct {
		Format          string
		WindowSize      int             `json:",omitempty"`
		LC              int             `json:",omitempty"`
		LP              int             `json:",omitempty"`
		PB              int             `json:",omitempty"`
		FixedProperties bool            `json:",omitempty"`
		Workers         int             `json:",omitempty"`
		WorkSize        int             `json:",omitempty"`
		ParserOptions   json.RawMessage `json:",omitempty"`
	}{}
	if err = json.Unmarshal(p, &s); err != nil {
		return err
	}
	if s.Format != "LZMA2" {
		return errors.New(
			"lzma: Format JSON property muse have value LZMA")
	}
	var parserOptions lz.Configurator
	if len(s.ParserOptions) > 0 {
		parserOptions, err = lz.UnmarshalJSONOptions(s.ParserOptions)
		if err != nil {
			return fmt.Errorf("lz.UnmarshalJSONOptions(%q): %w",
				s.ParserOptions, err)
		}
	}
	*cfg = Writer2Options{
		WindowSize: s.WindowSize,
		Properties: Properties{
			LC: s.LC,
			LP: s.LP,
			PB: s.PB,
		},
		FixedProperties: s.FixedProperties,
		Workers:         s.Workers,
		WorkSize:        s.WorkSize,
		ParserOptions:   parserOptions,
	}
	return nil
}

// MarshalJSON creates the JSON representation for the cfg value.
func (cfg *Writer2Options) MarshalJSON() (p []byte, err error) {
	s := struct {
		Format          string
		WindowSize      int             `json:",omitempty"`
		LC              int             `json:",omitempty"`
		LP              int             `json:",omitempty"`
		PB              int             `json:",omitempty"`
		FixedProperties bool            `json:",omitempty"`
		Workers         int             `json:",omitempty"`
		WorkSize        int             `json:",omitempty"`
		ParserOptions   lz.Configurator `json:",omitempty"`
	}{
		Format:          "LZMA2",
		WindowSize:      cfg.WindowSize,
		LC:              cfg.Properties.LC,
		LP:              cfg.Properties.LP,
		PB:              cfg.Properties.PB,
		FixedProperties: cfg.FixedProperties,
		Workers:         cfg.Workers,
		WorkSize:        cfg.WorkSize,
		ParserOptions:   cfg.ParserOptions,
	}
	return json.Marshal(&s)
}

// verify checks whether the configuration is consistent and correct. Usually
// call SetDefaults before this method.
func (cfg *Writer2Options) verify() error {
	var err error
	if cfg == nil {
		return errors.New("lzma: Writer2Options pointer must not be nil")
	}

	if cfg.ParserOptions == nil {
		return errors.New("lzma: Writer2Options field LZCfg is nil")
	}

	if err = cfg.Properties.Verify(); err != nil {
		return err
	}

	if cfg.Workers < 0 {
		return errors.New("lzma: Worker must be larger than 0")
	}

	if cfg.Workers > 1 && cfg.WorkSize <= 0 {
		return errors.New(
			"lzma: WorkerBufferSize must be greater than 0")
	}

	if cfg.Workers > 1 {
		bufferSize := lz.BufferSize(cfg.ParserOptions)
		if cfg.WorkSize > bufferSize {
			return errors.New(
				"lzma: sequence buffer size must be" +
					" less or equal than worker buffer size")
		}
	}

	return nil
}

// fixParserOptions computes the sequence buffer configuration in a way that
// works for lzma.
func fixParserOptions(cfg lz.Configurator, windowSize int) error {
	var err error
	if err = lz.SetWindowSize(cfg, windowSize); err != nil {
		return err
	}
	if err = lz.SetRetentionSize(cfg, windowSize); err != nil {
		return err
	}
	bufferSize := max(2*windowSize, 256<<10)
	if err = lz.SetBufferSize(cfg, bufferSize); err != nil {
		return err
	}
	return nil
}

// setDefaults replaces zero values with default values. The workers variable
// will be set to the number of CPUs.
func (cfg *Writer2Options) setDefaults() {
	if cfg.WindowSize == 0 {
		cfg.WindowSize = 8 << 20
	}
	if cfg.ParserOptions == nil {
		cfg.ParserOptions = presetParserOptions(5)
	}
	fixParserOptions(cfg.ParserOptions, cfg.WindowSize)

	var zeroProps = Properties{}
	if cfg.Properties == zeroProps && !cfg.FixedProperties {
		cfg.Properties = Properties{3, 0, 2}
	}

	if cfg.Workers == 0 {
		cfg.Workers = runtime.GOMAXPROCS(0)
	}

	if cfg.WorkSize == 0 && cfg.Workers > 1 {
		cfg.WorkSize = max(1<<20, 2*cfg.WindowSize)
		lz.SetBufferSize(cfg.ParserOptions, cfg.WorkSize)
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
	return NewWriter2Options(z, Writer2Options{})
}

// NewWriter2Options constructs an LZMA2 writer for a specific configuration.
// Note that the implementation for cfg.Workers > 1 uses go routines.
func NewWriter2Options(z io.Writer, cfg Writer2Options) (w Writer2, err error) {
	cfg.setDefaults()
	if err = cfg.verify(); err != nil {
		return nil, err
	}

	if cfg.Workers == 1 {
		parser, err := cfg.ParserOptions.NewParser()
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
		buf:    make([]byte, 0, cfg.WorkSize+7),
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
	cfg     Writer2Options
	err     error
}

func (w *mtWriter) DictSize() int {
	return lz.WindowSize(w.cfg.ParserOptions)
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
		k := w.cfg.WorkSize - len(w.buf)
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
		w.buf = make([]byte, 0, w.cfg.WorkSize+7)
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
		w.buf = make([]byte, 0, w.cfg.WorkSize+7)
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

func mtwWork(ctx context.Context, taskCh <-chan mtwTask, cfg Writer2Options) {
	parser, err := cfg.ParserOptions.NewParser()
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
