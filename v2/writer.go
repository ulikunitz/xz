// SPDX-FileCopyrightText: © 2014 Ulrich Kunitz
//
// SPDX-License-Identifier: BSD-3-Clause

package xz

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"hash"
	"io"
	"runtime"

	"github.com/ulikunitz/lz"
	"github.com/ulikunitz/xz/v2/lzma"
)

// defaultParallelBlockSize defines the default block size for more than 1
// worker as 256 kByte.
const defaultParallelBlockSize = 256 << 10

// maxInt64 defines the maximum 64-bit signed integer.
const maxInt64 = 1<<63 - 1

// WriterOptions describe the parameters for an xz writer. CRC64 is used as the
// default checksum despite the XZ specification saying a decoder must only
// support CRC32.
type WriterOptions struct {
	// WindowSize sets the dictionary size.
	WindowSize int
	// BufferSize sets the size of the buffer used by the LZ parser.
	BufferSize int

	// Properties for the LZMA algorithm.
	Properties lzma.Properties
	// FixedProperties indicate that the Properties is indeed zero
	FixedProperties bool

	// Number of workers processing data.
	Workers int
	// LZMAParallel indicates that the parallel execution should be on the
	// LZMA level. (This is an experimental setup and should normally not be
	// used.)
	LZMAParallel bool
	// Size of buffer used by the worker in LZMA work.
	LZMAWorkSize int

	// Configuration for the LZ parser.
	ParserOptions lz.Configurator

	// XZBlockSize defines the maximum uncompressed size of a xz-format
	// block. The default for a single worker setup MaxInt64=2^63-1 and 256
	// kByte with multiple parallel workers. Note that the XZ block size
	// differs from the parser block size.
	XZBlockSize int64

	// checksum method: CRC32, CRC64 or SHA256 (default: CRC64)
	Checksum byte

	// Forces NoChecksum (default: false)
	NoChecksum bool
}

type checksum byte

func (c *checksum) UnmarshalText(text []byte) error {
	switch string(text) {
	case "<none>":
		*c = 0
	case "crc32":
		*c = checksum(CRC32)
	case "crc64":
		*c = checksum(CRC64)
	case "sha256":
		*c = checksum(SHA256)
	default:
		*c = 0
		return fmt.Errorf("xz: unsupported checksum value %q", text)
	}
	return nil
}

func (c *checksum) MarshalText() (data []byte, err error) {
	switch byte(*c) {
	case 0:
		data = []byte("<none>")
	case CRC32:
		data = []byte("crc32")
	case CRC64:
		data = []byte("crc64")
	case SHA256:
		data = []byte("sha256")
	default:
		return nil, fmt.Errorf("xz: unsupported checksum value %#02x",
			*c)
	}
	return data, nil
}

// UnmarshalJSON parses a JSON value and set the WriterOptions value accordingly.
func (opts *WriterOptions) UnmarshalJSON(p []byte) error {
	var err error
	s := struct {
		Format          string
		WindowSize      int             `json:",omitempty"`
		LC              int             `json:",omitempty"`
		LP              int             `json:",omitempty"`
		PB              int             `json:",omitempty"`
		FixedProperties bool            `json:",omitempty"`
		Workers         int             `json:",omitempty"`
		LZMAParallel    bool            `json:",omitempty"`
		LZMAWorkSize    int             `json:",omitempty"`
		ParserOptions   json.RawMessage `json:",omitempty"`
		XZBlockSize     int64           `json:",omitempty"`
		Checksum        checksum        `json:",omitempty"`
		NoChecksum      bool            `json:",omitempty"`
	}{}
	if err = json.Unmarshal(p, &s); err != nil {
		return err
	}
	if s.Format != "XZ" {
		return errors.New(
			"xz: Format JSON property muse have value XZ")
	}
	var parserOptions lz.Configurator
	if len(s.ParserOptions) > 0 {
		parserOptions, err = lz.ParseJSON(s.ParserOptions)
		if err != nil {
			return fmt.Errorf("lz.ParseJSON(%q): %w",
				s.ParserOptions, err)
		}
	}
	if err != nil {
		return fmt.Errorf("xz.WriterOptions.UnmarshalJSON: %w", err)
	}
	*opts = WriterOptions{
		WindowSize: s.WindowSize,
		Properties: lzma.Properties{
			LC: s.LC,
			LP: s.LP,
			PB: s.PB,
		},
		FixedProperties: s.FixedProperties,
		Workers:         s.Workers,
		LZMAParallel:    s.LZMAParallel,
		LZMAWorkSize:    s.LZMAWorkSize,
		ParserOptions:   parserOptions,
		XZBlockSize:     s.XZBlockSize,
		Checksum:        byte(s.Checksum),
		NoChecksum:      s.NoChecksum,
	}
	return nil
}

// MarshalJSON creates the JSON representation of the WriterOptions value.
func (opts *WriterOptions) MarshalJSON() (p []byte, err error) {
	s := struct {
		Format          string
		WindowSize      int             `json:",omitempty"`
		LC              int             `json:",omitempty"`
		LP              int             `json:",omitempty"`
		PB              int             `json:",omitempty"`
		FixedProperties bool            `json:",omitempty"`
		Workers         int             `json:",omitempty"`
		LZMAParallel    bool            `json:",omitempty"`
		LZMAWorkSize    int             `json:",omitempty"`
		ParserOptions   lz.Configurator `json:",omitempty"`
		XZBlockSize     int64           `json:",omitempty"`
		Checksum        checksum        `json:",omitempty"`
		NoChecksum      bool            `json:",omitempty"`
	}{
		Format:          "XZ",
		WindowSize:      opts.WindowSize,
		LC:              opts.Properties.LC,
		LP:              opts.Properties.LP,
		PB:              opts.Properties.PB,
		FixedProperties: opts.FixedProperties,
		Workers:         opts.Workers,
		LZMAParallel:    opts.LZMAParallel,
		LZMAWorkSize:    opts.LZMAWorkSize,
		ParserOptions:   opts.ParserOptions,
		XZBlockSize:     opts.XZBlockSize,
		Checksum:        checksum(opts.Checksum),
		NoChecksum:      opts.NoChecksum,
	}
	return json.Marshal(&s)
}

// setDefaults applies the defaults to the xz writer configuration.
func (opts *WriterOptions) setDefaults() {

	preset := Preset(5)
	if opts.WindowSize == 0 {
		opts.WindowSize = preset.WindowSize
	}
	if opts.Properties == (lzma.Properties{}) {
		opts.Properties = preset.Properties
	}
	if opts.Workers == 0 {
		opts.Workers = runtime.GOMAXPROCS(0)
	}
	if opts.Workers <= 1 {
		opts.XZBlockSize = maxInt64
	} else {
		opts.XZBlockSize = defaultParallelBlockSize
	}
	if opts.Checksum == 0 {
		opts.Checksum = CRC64
	}
	if opts.NoChecksum {
		opts.Checksum = None
	}
}

// verify checks the configuration for errors. Zero values will be
// replaced by default values.
func (opts *WriterOptions) verify() error {
	if opts == nil {
		return errors.New("xz: writer configuration is nil")
	}
	if !opts.LZMAParallel {
		if !(1 <= opts.Workers) {
			return errors.New("xz: Workers must be positive")
		}
	}
	if opts.XZBlockSize <= 0 {
		return errors.New("xz: block size out of range")
	}
	if err := verifyFlags(opts.Checksum); err != nil {
		return err
	}
	return nil
}

// filters creates the filter list for the given parameters.
func filters(cfg *WriterOptions) []filter {
	return []filter{&lzmaFilter{
		int64(cfg.WindowSize)}}
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
func newFilterWriteCloser(w io.Writer, f []filter, c *WriterOptions) (fw io.WriteCloser, err error) {
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
	cfg WriterOptions

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

func newBlockWriter(w io.Writer, cfg *WriterOptions) (bw *blockWriter, err error) {

	h, err := newHash(cfg.Checksum)
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
	k := bw.cfg.XZBlockSize - bw.n
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
			"xz: header can only be provided if blockWriter is closed")
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

// WriteFlushCloser supports the Write, Flush and Close methods.
type WriteFlushCloser interface {
	io.WriteCloser
	Flush() error
}

// NewWriter creates a new Writer for xz-compressed data. The Writer uses the
// preset #5. See [Preset] and [NewWriterOptions] for changing the parameters.
func NewWriter(xz io.Writer) (w WriteFlushCloser, err error) {
	return NewWriterOptions(xz, Preset(5))
}

// NewWriterOptions creates a WriteFlushCloser instance. If multi-threading is
// requested by a Workers configuration larger than 1, single threading will be
// requested for the LZMA writer by setting the Workers variable there to 1.
func NewWriterOptions(xz io.Writer, options WriterOptions) (w WriteFlushCloser, err error) {
	options.setDefaults()
	if err = options.verify(); err != nil {
		return nil, err
	}

	if options.Workers <= 1 || options.LZMAParallel {
		return newStreamWriter(xz, &options)
	}

	return newMTWriter(xz, &options)
}

type streamWriter struct {
	cfg WriterOptions

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

func newStreamWriter(xz io.Writer, cfg *WriterOptions) (sw *streamWriter, err error) {
	_, err = writeHeader(xz, cfg.Checksum)
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
	if _, err = writeTail(sw.xz, sw.index, sw.cfg.Checksum); err != nil {
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
	opts WriterOptions

	ctx      context.Context
	cancel   context.CancelFunc
	errCh    chan error
	taskCh   chan mtwTask
	streamCh chan mtwStreamTask

	buf     []byte
	workers int
	err     error
}

func newMTWriter(xz io.Writer, options *WriterOptions) (mtw *mtWriter, err error) {
	ctx, cancel := context.WithCancel(context.Background())
	mtw = &mtWriter{
		opts: *options,

		ctx:      ctx,
		cancel:   cancel,
		errCh:    make(chan error, 1),
		taskCh:   make(chan mtwTask, options.Workers),
		streamCh: make(chan mtwStreamTask, options.Workers),

		buf: make([]byte, 0, options.XZBlockSize),
	}

	go mtwStream(ctx, xz, options, mtw.streamCh, mtw.errCh)

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
		k := mtw.opts.XZBlockSize - int64(len(mtw.buf))
		if int64(len(p)) < k {
			mtw.buf = append(mtw.buf, p...)
			n += len(p)
			return n, nil
		}
		mtw.buf = append(mtw.buf, p[:k]...)

		if mtw.workers < mtw.opts.Workers {
			go mtwWorker(mtw.ctx, &mtw.opts, mtw.taskCh, mtw.errCh)
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
		mtw.buf = make([]byte, 0, mtw.opts.XZBlockSize)
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
		if mtw.workers < mtw.opts.Workers {
			go mtwWorker(mtw.ctx, &mtw.opts, mtw.taskCh, mtw.errCh)
			mtw.workers++
		}
		blockCh = make(chan mtwBlock, 1)
		select {
		case mtw.taskCh <- mtwTask{buf: mtw.buf, blockCh: blockCh}:
		case err = <-mtw.errCh:
			recv(err)
			return err
		}
		mtw.buf = make([]byte, 0, mtw.opts.XZBlockSize)
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

func mtwStream(ctx context.Context, xz io.Writer, cfg *WriterOptions,
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
	_, err := writeHeader(xz, cfg.Checksum)
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
			_, err = writeTail(xz, index, cfg.Checksum)
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

func mtwWorker(ctx context.Context, cfg *WriterOptions, taskCh <-chan mtwTask,
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
