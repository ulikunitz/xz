// Copyright 2014-2021 Ulrich Kunitz. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package xz

import (
	"bytes"
	"context"
	"errors"
	"hash"
	"io"
	"runtime"

	"github.com/ulikunitz/xz/lzma"
)

// defaultParallelBlockSize defines the default block size for more than 1
// worker as 256 kbyte.
const defaultParallelBlockSize = 256 << 10

// maxInt64 defines the maximum 64-bit signed integer.
const maxInt64 = 1<<63 - 1

// WriterConfig describe the parameters for an xz writer. CRC64 is used as the
// default checksum despite the XZ specification saying a decoder must only
// support CRC32.
type WriterConfig struct {
	// LZMA2 configuration
	LZMACfg lzma.Writer2Config

	// BlockSize defines the maximum uncompressed size of a block.
	// (default: MaxInt64=2^63-1) if Worker equals 1 or 8 MB otherwise.
	BlockSize int64

	// checksum method: CRC32, CRC64 or SHA256 (default: CRC64)
	CheckSum byte
	// Forces NoChecksum (default: false)
	NoCheckSum bool

	// Workers defines the number of goroutines compressing data. If it is
	// zero the GONUMPROCS environment variable determines the number of
	// goroutines. If the number is 1 the compression happens in classic
	// streaming mode and the compressed file must be also decompressed
	// serially.
	Workers int
}

// ApplyDefaults applies the defaults to the xz writer configuration.
func (c *WriterConfig) ApplyDefaults() {
	c.LZMACfg.Workers = 1
	c.LZMACfg.WorkerBufferSize = 0
	c.LZMACfg.ApplyDefaults()
	if c.CheckSum == 0 {
		c.CheckSum = CRC64
	}
	if c.NoCheckSum {
		c.CheckSum = None
	}
	if c.Workers == 0 {
		c.Workers = runtime.GOMAXPROCS(0)
	}
	if c.BlockSize == 0 {
		if c.Workers <= 1 {
			c.BlockSize = maxInt64
		} else {
			c.BlockSize = defaultParallelBlockSize
		}
	}
}

// Verify checks the configuration for errors. Zero values will be
// replaced by default values.
func (c *WriterConfig) Verify() error {
	if c == nil {
		return errors.New("xz: writer configuration is nil")
	}
	c.ApplyDefaults()
	var err error
	if err = c.LZMACfg.Verify(); err != nil {
		return err
	}
	if c.BlockSize <= 0 {
		return errors.New("xz: block size out of range")
	}
	if err = verifyFlags(c.CheckSum); err != nil {
		return err
	}
	if !(1 <= c.Workers) {
		return errors.New("xz: Workers must be positive")
	}
	return nil
}

// filters creates the filter list for the given parameters.
func filters(c *WriterConfig) []filter {
	return []filter{&lzmaFilter{
		int64(c.LZMACfg.LZCfg.BufferConfig().WindowSize)}}
}

// verifyFilters checks the filter list for the length and the right
// sequence of filters.
func verifyFilters(f []filter) error {
	if len(f) == 0 {
		return errors.New("xz: no filters")
	}
	if len(f) > 4 {
		return errors.New("xz: more than four filters")
	}
	for _, g := range f[:len(f)-1] {
		if g.last() {
			return errors.New("xz: last filter is not last")
		}
	}
	if !f[len(f)-1].last() {
		return errors.New("xz: wrong last filter")
	}
	return nil
}

// newFilterWriteCloser converts a filter list into a WriteCloser that
// can be used by a blockWriter.
func newFilterWriteCloser(w io.Writer, f []filter, c *WriterConfig) (fw io.WriteCloser, err error) {
	fw = nopWriteCloser(w)
	for i := len(f) - 1; i >= 0; i-- {
		fw, err = f[i].writeCloser(fw, c)
		if err != nil {
			return nil, err
		}
	}
	return fw, nil
}

// nopWCloser implements a WriteCloser with a Close method not doing
// anything.
type nopWCloser struct {
	io.Writer
}

// Close returns nil and doesn't do anything else.
func (c nopWCloser) Close() error {
	return nil
}

// nopWriteCloser converts the Writer into a WriteCloser with a Close
// function that does nothing beside returning nil.
func nopWriteCloser(w io.Writer) io.WriteCloser { return nopWCloser{w} }

type countWriter struct {
	w io.Writer
	n int64
}

func (cw *countWriter) Write(p []byte) (n int, err error) {
	n, err = cw.w.Write(p[n:])
	cw.n += int64(n)
	return n, err
}

var errNoSpace = errors.New("xz: no space")

var errWriterClosed = errors.New("xz: writer is closed")

type blockWriter struct {
	cfg WriterConfig

	// filter array
	f []filter

	xz      io.Writer
	cw      countWriter
	fwc     io.WriteCloser
	hash    hash.Hash
	mw      io.Writer
	n       int64
	hdrSize int

	err error
}

func newBlockWriter(w io.Writer, cfg *WriterConfig) (bw *blockWriter, err error) {

	h, err := newHash(cfg.CheckSum)
	if err != nil {
		return nil, err
	}
	bw = &blockWriter{
		cfg:  *cfg,
		f:    filters(cfg),
		xz:   w,
		cw:   countWriter{w: w},
		hash: h,
	}

	if err = bw.reset(nil); err != nil {
		return nil, err
	}
	return bw, nil
}

func (bw *blockWriter) reset(xz io.Writer) error {
	if bw.err != nil && bw.err != errWriterClosed {
		return bw.err
	}
	bw.err = nil

	if xz != nil {
		bw.xz = xz
		bw.cw.w = xz
	}

	bw.hdrSize = 0

	bw.cw.n = 0
	var err error
	bw.fwc, err = newFilterWriteCloser(&bw.cw, bw.f, &bw.cfg)
	if err != nil {
		bw.err = err
		return err
	}
	bw.hash.Reset()
	bw.mw = io.MultiWriter(bw.fwc, bw.hash)
	bw.n = 0
	return nil
}

func (bw *blockWriter) writeHeaderStreaming() error {
	if bw.cfg.Workers > 1 {
		return nil
	}
	hdr := blockHeader{
		compressedSize:   -1,
		uncompressedSize: -1,
		filters:          bw.f,
	}
	data, err := hdr.MarshalBinary()
	if err != nil {
		bw.err = err
		return err
	}
	bw.hdrSize, err = bw.xz.Write(data)
	if err != nil {
		bw.err = err
		return err
	}
	return nil
}

func (bw *blockWriter) Write(p []byte) (n int, err error) {
	if bw.err != nil {
		return 0, bw.err
	}
	k := bw.cfg.BlockSize - bw.n
	if k < int64(len(p)) {
		p = p[:k]
		err = errNoSpace
	}
	if len(p) == 0 {
		return n, err
	}
	if bw.hdrSize == 0 && bw.cfg.Workers <= 1 {
		if err = bw.writeHeaderStreaming(); err != nil {
			return n, err
		}
	}
	var werr error
	n, werr = bw.mw.Write(p)
	if werr != nil {
		err = werr
	}
	bw.n += int64(n)
	bw.err = err
	return n, err
}

var errNoBlock = errors.New("xz: no data in block")

func (bw *blockWriter) Close() error {
	if bw.err != nil && bw.err != errNoSpace {
		return bw.err
	}
	if bw.n == 0 && bw.hdrSize == 0 {
		bw.err = nil
		return errNoBlock
	}
	var err error
	if err = bw.fwc.Close(); err != nil {
		bw.err = err
		return err
	}
	k := padLen(bw.cw.n)
	p := make([]byte, k, k+bw.hash.Size())
	p = bw.hash.Sum(p)
	if _, err := bw.xz.Write(p); err != nil {
		bw.err = err
		return err
	}
	bw.err = errWriterClosed
	return nil
}

func (bw *blockWriter) appendHeaderAfterClose(in []byte) (p []byte, err error) {
	p = in
	if bw.err != errWriterClosed {
		return p, errors.New(
			"xz: header can pnly be provided if blockWriter is closed")
	}
	if bw.cfg.Workers <= 1 {
		return p, errors.New("xz: header already written")
	}
	hdr := blockHeader{
		compressedSize:   bw.cw.n,
		uncompressedSize: bw.n,
		filters:          bw.f,
	}
	q, err := hdr.MarshalBinary()
	if err != nil {
		return p, err
	}
	bw.hdrSize = len(q)
	p = append(p, q...)
	return p, nil
}

func (bw *blockWriter) record() (r record, err error) {
	if bw.err != errWriterClosed {
		return r, errors.New(
			"xz: record can nly be provided if blockWriter is closed")
	}
	if bw.hdrSize == 0 {
		return r, errors.New("xz: header not created")
	}
	r.unpaddedSize = int64(bw.hdrSize) + bw.cw.n + int64(bw.hash.Size())
	r.uncompressedSize = bw.n
	return r, nil
}

type WriteFlushCloser interface {
	io.WriteCloser
	Flush() error
}

func NewWriter(xz io.Writer) (w WriteFlushCloser, err error) {
	return NewWriterConfig(xz, WriterConfig{})
}

func NewWriterConfig(xz io.Writer, cfg WriterConfig) (w WriteFlushCloser, err error) {
	cfg.ApplyDefaults()
	if err = cfg.Verify(); err != nil {
		return nil, err
	}

	if cfg.Workers <= 1 {
		return newStreamWriter(xz, &cfg)
	}

	return newMTWriter(xz, &cfg)
}

type streamWriter struct {
	cfg WriterConfig

	xz    io.Writer
	bw    *blockWriter
	index []record

	err error
}

func writeHeader(xz io.Writer, flags byte) (n int, err error) {
	hdr := header{flags: flags}
	p, err := hdr.MarshalBinary()
	if err != nil {
		return 0, err
	}
	return xz.Write(p)
}

func writeTail(xz io.Writer, index []record, flags byte) (n int64, err error) {
	f := footer{flags: flags}
	f.indexSize, err = writeIndex(xz, index)
	n += f.indexSize
	if err != nil {
		return n, err
	}
	p, err := f.MarshalBinary()
	if err != nil {
		return n, err
	}
	k, err := xz.Write(p)
	n += int64(k)
	return n, err
}

func newStreamWriter(xz io.Writer, cfg *WriterConfig) (sw *streamWriter, err error) {
	_, err = writeHeader(xz, cfg.CheckSum)
	if err != nil {
		return nil, err
	}
	bw, err := newBlockWriter(xz, cfg)
	if err != nil {
		return nil, err
	}
	sw = &streamWriter{
		cfg: *cfg,
		xz:  xz,
		bw:  bw,
	}
	return sw, nil
}

func (sw *streamWriter) Write(p []byte) (n int, err error) {
	if sw.err != nil {
		return 0, sw.err
	}
	for n < len(p) {
		k, err := sw.bw.Write(p[n:])
		n += k
		if err != errNoSpace {
			if err != nil {
				sw.err = err
			}
			return n, err
		}
		if err = sw.Flush(); err != nil {
			return n, err
		}
	}
	return n, nil
}

func (sw *streamWriter) Close() error {
	if sw.err != nil {
		return sw.err
	}
	var err error
	if err = sw.Flush(); err != nil {
		return err
	}
	if _, err = writeTail(sw.xz, sw.index, sw.cfg.CheckSum); err != nil {
		sw.err = err
		return err
	}
	sw.err = errWriterClosed
	return nil
}

func (sw *streamWriter) Flush() error {
	if sw.err != nil {
		return sw.err
	}
	var err error
	if err = sw.bw.Close(); err != nil {
		if err == errNoBlock {
			return nil
		}
		sw.err = err
		return err
	}
	r, err := sw.bw.record()
	if err != nil {
		sw.err = err
		return err
	}
	sw.index = append(sw.index, r)
	err = sw.bw.reset(nil)
	if err != nil {
		sw.err = err
		return err
	}
	return err
}

type mtwStreamTask struct {
	blockCh chan mtwBlock
	flushCh chan struct{}
	close   bool
}

type mtwBlock struct {
	hdr  []byte
	body []byte
	rec  record
}

type mtwTask struct {
	buf     []byte
	blockCh chan<- mtwBlock
}

type mtWriter struct {
	cfg WriterConfig

	ctx      context.Context
	cancel   context.CancelFunc
	errCh    chan error
	taskCh   chan mtwTask
	streamCh chan mtwStreamTask

	buf     []byte
	workers int
	err     error
}

func newMTWriter(xz io.Writer, cfg *WriterConfig) (mtw *mtWriter, err error) {
	ctx, cancel := context.WithCancel(context.Background())
	mtw = &mtWriter{
		cfg: *cfg,

		ctx:      ctx,
		cancel:   cancel,
		errCh:    make(chan error, 1),
		taskCh:   make(chan mtwTask, cfg.Workers),
		streamCh: make(chan mtwStreamTask, cfg.Workers),

		buf: make([]byte, 0, cfg.BlockSize),
	}

	go mtwStream(ctx, xz, cfg, mtw.streamCh, mtw.errCh)

	return mtw, nil
}

func (mtw *mtWriter) Write(p []byte) (n int, err error) {
	if mtw.err != nil {
		return 0, mtw.err
	}

	recv := func(err error) {
		if err == nil {
			panic("nil error from errCh")
		}
		mtw.err = err
		mtw.cancel()
	}

	for len(p) > 0 {
		k := mtw.cfg.BlockSize - int64(len(mtw.buf))
		if int64(len(p)) < k {
			mtw.buf = append(mtw.buf, p...)
			n += len(p)
			return n, nil
		}
		mtw.buf = append(mtw.buf, p[:k]...)

		if mtw.workers < mtw.cfg.Workers {
			go mtwWorker(mtw.ctx, &mtw.cfg, mtw.taskCh, mtw.errCh)
			mtw.workers++
		}

		blockCh := make(chan mtwBlock, 1)
		select {
		case mtw.taskCh <- mtwTask{buf: mtw.buf, blockCh: blockCh}:
		case err = <-mtw.errCh:
			recv(err)
			return n, err
		}
		select {
		case mtw.streamCh <- mtwStreamTask{blockCh: blockCh}:
		case err = <-mtw.errCh:
			recv(err)
			return n, err
		}
		n += int(k)
		p = p[k:]
		mtw.buf = make([]byte, 0, mtw.cfg.BlockSize)
	}

	return n, nil
}

func (mtw *mtWriter) flush(close bool) error {
	if mtw.err != nil {
		return mtw.err
	}

	recv := func(err error) {
		if err == nil {
			panic("nil error from errCh")
		}
		mtw.err = err
		mtw.cancel()
	}

	var (
		err     error
		blockCh chan mtwBlock
	)

	if len(mtw.buf) > 0 {
		if mtw.workers < mtw.cfg.Workers {
			go mtwWorker(mtw.ctx, &mtw.cfg, mtw.taskCh, mtw.errCh)
			mtw.workers++
		}
		blockCh = make(chan mtwBlock, 1)
		select {
		case mtw.taskCh <- mtwTask{buf: mtw.buf, blockCh: blockCh}:
		case err = <-mtw.errCh:
			recv(err)
			return err
		}
		mtw.buf = make([]byte, 0, mtw.cfg.BlockSize)
	}

	flushCh := make(chan struct{})
	select {
	case mtw.streamCh <- mtwStreamTask{
		blockCh: blockCh,
		flushCh: flushCh,
		close:   close,
	}:
	case err = <-mtw.errCh:
		recv(err)
		return err
	}

	select {
	case <-flushCh:
	case err = <-mtw.errCh:
		recv(err)
		return err
	}

	return nil
}

func (mtw *mtWriter) Flush() error {
	return mtw.flush(false)
}

func (mtw *mtWriter) Close() error {
	if err := mtw.flush(true); err != nil {
		return err
	}

	mtw.cancel()
	mtw.err = errWriterClosed
	return nil
}

func mtwStream(ctx context.Context, xz io.Writer, cfg *WriterConfig,
	streamCh <-chan mtwStreamTask, errCh chan<- error) {

	send := func(err error) (stop bool) {
		select {
		case errCh <- err:
			return false
		case <-ctx.Done():
			return true
		}
	}

	var index []record
	_, err := writeHeader(xz, cfg.CheckSum)
	if err != nil {
		send(err)
		return
	}

	for {
		var tsk mtwStreamTask

		select {
		case <-ctx.Done():
			return
		case tsk = <-streamCh:
		}

		if tsk.blockCh != nil {
			var block mtwBlock
			select {
			case <-ctx.Done():
				return
			case block = <-tsk.blockCh:
			}
			if _, err = xz.Write(block.hdr); err != nil {
				send(err)
				return
			}
			if _, err = xz.Write(block.body); err != nil {
				send(err)
				return
			}
			index = append(index, block.rec)
		}

		if tsk.close {
			_, err = writeTail(xz, index, cfg.CheckSum)
			if err != nil {
				send(err)
			}
		}

		if tsk.flushCh != nil {
			select {
			case <-ctx.Done():
				return
			case tsk.flushCh <- struct{}{}:
			}
		}

		if tsk.close {
			return
		}
	}
}

func mtwWorker(ctx context.Context, cfg *WriterConfig, taskCh <-chan mtwTask,
	errCh chan<- error) {

	send := func(err error) (stop bool) {
		select {
		case errCh <- err:
			return false
		case <-ctx.Done():
			return true
		}
	}

	bw, err := newBlockWriter(nil, cfg)
	if err != nil {
		send(err)
		return
	}

	for {
		var tsk mtwTask
		select {
		case <-ctx.Done():
			return
		case tsk = <-taskCh:
		}

		buf := new(bytes.Buffer)
		if err = bw.reset(buf); err != nil {
			send(err)
			return
		}

		if _, err = bw.Write(tsk.buf); err != nil {
			send(err)
			return
		}
		if err = bw.Close(); err != nil {
			send(err)
			return
		}

		var blk mtwBlock
		if blk.hdr, err = bw.appendHeaderAfterClose(nil); err != nil {
			send(err)
			return
		}
		blk.body = buf.Bytes()
		if blk.rec, err = bw.record(); err != nil {
			send(err)
			return
		}
		select {
		case <-ctx.Done():
			return
		case tsk.blockCh <- blk:
		}
	}
}
