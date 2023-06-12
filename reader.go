// SPDX-FileCopyrightText: © 2014 Ulrich Kunitz
//
// SPDX-License-Identifier: BSD-3-Clause

// Package xz supports the compression and decompression of xz files. It
// supports version 1.1.0 of the specification without the non-LZMA2
// filters. See http://tukaani.org/xz/xz-file-format-1.1.0.txt
package xz

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"hash"
	"io"
	"runtime"

	"github.com/ulikunitz/xz/lzma"
)

var errReaderClosed = errors.New("xz: reader closed")

// ReaderConfig defines the parameters for the xz reader. The SingleStream
// parameter requests the reader to assume that the underlying stream contains
// only a single stream without padding.
//
// The workers variable controls the number of parallel workers decoding the
// file. It only has an effect if the file was encoded in a way that it created
// blocks with the compressed size set in the headers. If Workers not 1 the
// Workers variable in LZMAConfig will be ignored.
type ReaderConfig struct {
	// Workers defines the number of readers for parallel reading. The
	// default is the value of GOMAXPROCS.
	Workers int

	// Read a single xz stream from the underlying reader, stop and return
	// EOF. No checks are done whether the underlying reader finishes too.
	SingleStream bool

	// Runs the multiple Workers in LZMA mode. (This is an experimental
	// setup is normally not required.)
	LZMAParallel bool

	// LZMAWorkSize provides the work size to the LZMA layer. It is only
	// required if LZMAParallel is set.
	LZMAWorkSize int
}

// UnmarshalJSON parses JSON and sets the ReaderConfig accordingly.
func (cfg *ReaderConfig) UnmarshalJSON(p []byte) error {
	var err error
	s := struct {
		Format       string
		Type         string
		Workers      int
		SingleStream bool
		LZMAParallel bool
		LZMAWorkSize int
	}{}
	if err = json.Unmarshal(p, &s); err != nil {
		return err
	}
	if s.Format != "XZ" {
		return errors.New(
			"xz: Format JSON property must have value XZ")
	}
	if s.Type != "Reader" {
		return errors.New(
			"xz: Type JSON property must have value Reader")
	}
	*cfg = ReaderConfig{
		Workers:      s.Workers,
		SingleStream: s.SingleStream,
		LZMAParallel: s.LZMAParallel,
		LZMAWorkSize: s.LZMAWorkSize,
	}
	return nil
}

// MarshalJSON creates the jason structure for a ReaderConfig value.
func (cfg *ReaderConfig) MarshalJSON() (p []byte, err error) {
	s := struct {
		Format       string
		Type         string
		Workers      int  `json:",omitempty"`
		SingleStream bool `json:",omitempty"`
		LZMAParallel bool `json:",omitempty"`
		LZMAWorkSize int  `json:",omitempty"`
	}{
		Format:       "XZ",
		Type:         "Reader",
		Workers:      cfg.Workers,
		SingleStream: cfg.SingleStream,
		LZMAParallel: cfg.LZMAParallel,
		LZMAWorkSize: cfg.LZMAWorkSize,
	}
	return json.Marshal(&s)
}

// SetDefaults sets the defaults in ReaderConfig.
func (cfg *ReaderConfig) SetDefaults() {
	if cfg.LZMAParallel {
		lzmaCfg := lzma.Reader2Config{
			Workers:  cfg.Workers,
			WorkSize: cfg.LZMAWorkSize,
		}
		lzmaCfg.SetDefaults()
		cfg.Workers = lzmaCfg.Workers
		cfg.LZMAWorkSize = lzmaCfg.WorkSize
	} else {
		if cfg.Workers == 0 {
			cfg.Workers = runtime.GOMAXPROCS(0)
		}
	}
}

// Verify checks the reader parameters for Validity. Zero values will be
// replaced by default values.
func (cfg *ReaderConfig) Verify() error {
	if cfg == nil {
		return errors.New("xz: reader parameters are nil")
	}

	var lzmaCfg lzma.Reader2Config
	if cfg.LZMAParallel {
		lzmaCfg = lzma.Reader2Config{
			Workers:  cfg.Workers,
			WorkSize: cfg.LZMAWorkSize,
		}
	} else {
		if cfg.Workers < 1 {
			return errors.New("xz: reader workers must be >= 1")
		}
		lzmaCfg = lzma.Reader2Config{
			Workers:  1,
			WorkSize: cfg.LZMAWorkSize,
		}
	}
	lzmaCfg.SetDefaults()
	if err := lzmaCfg.Verify(); err != nil {
		return err
	}

	return nil
}

// newFilterReader constructs the reader for the given filter.
func (cfg *ReaderConfig) newFilterReader(r io.Reader, f []filter) (fr io.ReadCloser, err error) {

	if err = verifyFilters(f); err != nil {
		return nil, err
	}

	fr = io.NopCloser(r)
	for i := len(f) - 1; i >= 0; i-- {
		fr, err = f[i].reader(fr, cfg)
		if err != nil {
			return nil, err
		}
	}
	return fr, nil
}

// streamReader defines the interface to the streamReader implementation. We
// have the single-threader stream reader [stReader] and the multi-threaded
// reader [mtReader].
type streamReader interface {
	io.ReadCloser
	reset(hdr *header) error
}

// reader supports the reading of one or multiple xz streams.
type reader struct {
	cfg ReaderConfig

	xz io.Reader
	sr streamReader

	err error
}

// NewReader creates an io.ReadCloser. The function should never fail.
func NewReader(xz io.Reader) (r io.ReadCloser, err error) {
	r, err = NewReaderConfig(xz, ReaderConfig{})
	if err != nil {
		return nil, err
	}
	return r, nil
}

// NewReaderConfig creates an xz reader using the provided configuration. If
// Workers are larger than one, the LZMA reader will only use single-threaded
// workers.
func NewReaderConfig(xz io.Reader, cfg ReaderConfig) (r io.ReadCloser, err error) {
	cfg.SetDefaults()
	if err = cfg.Verify(); err != nil {
		return nil, err
	}

	rp := &reader{cfg: cfg}

	if cfg.Workers <= 1 || cfg.LZMAParallel {
		// for the single thread reader we are buffering
		rp.xz = bufio.NewReader(xz)
		rp.sr = newSingleThreadStreamReader(rp.xz, &rp.cfg)
	} else {
		rp.xz = xz
		rp.sr = newMultiThreadStreamReader(rp.xz, &rp.cfg)
	}

	// read header without padding
	hdr, err := readHeader(rp.xz, false)
	if err != nil {
		return nil, err
	}
	if err = rp.sr.reset(hdr); err != nil {
		return nil, err
	}
	return rp, err
}

// Read reads the uncompressed data.
func (r *reader) Read(p []byte) (n int, err error) {
	if r.err != nil {
		return 0, r.err
	}
	for n < len(p) {
		k, err := r.sr.Read(p[n:])
		n += k
		if err != nil {
			if err == io.EOF {
				if err = r.sr.Close(); err != nil {
					r.err = err
					return n, err
				}
				if r.cfg.SingleStream {
					// return simply with EOF after a single
					// stream is read. Checking for EOF in
					// the underlying reader can be done by
					// the client code.
					r.err = io.EOF
					return n, nil
				}
				// read header with padding
				hdr, err := readHeader(r.xz, true)
				if err != nil {
					r.err = err
					return n, err
				}
				if err = r.sr.reset(hdr); err != nil {
					r.err = err
					return n, err
				}
				continue
			}
			r.err = err
			return n, err
		}
	}
	return n, nil
}

// Close closes the reader an releases underlying resources, especially the the
// multithreaded tasks.
func (r *reader) Close() error {
	if r.err == errReaderClosed {
		return errReaderClosed
	}
	if err := r.sr.Close(); err != nil && err != errReaderClosed {
		r.err = err
		return err
	}
	r.err = errReaderClosed
	return nil
}

// countingReader is a reader that counts the bytes read.
type countingReader struct {
	r io.Reader
	n int64
}

// Read reads data from the wrapped reader and adds it to the n field.
func (lr *countingReader) Read(p []byte) (n int, err error) {
	n, err = lr.r.Read(p)
	lr.n += int64(n)
	return n, err
}

// blockReader supports the reading of a block.
type blockReader struct {
	cfg *ReaderConfig

	hash hash.Hash

	header    *blockHeader
	headerLen int

	xz           io.Reader
	cxz          countingReader
	fr           io.ReadCloser
	r            io.Reader
	uncompressed int64

	err error
}

// init initializes the block reader.
func (br *blockReader) init(xz io.Reader, cfg *ReaderConfig, h hash.Hash) {
	*br = blockReader{
		cfg:  cfg,
		xz:   xz,
		hash: h,
	}
	h.Reset()
}

// reset resets the block reader to the status after init.
func (br *blockReader) reset() {
	*br = blockReader{
		cfg:  br.cfg,
		xz:   br.xz,
		hash: br.hash,
	}
	br.hash.Reset()
}

// setHeader sets the header for the block reader. It can only be called once
// after init or reset.
func (br *blockReader) setHeader(hdr *blockHeader, hdrLen int) error {
	if br.err != nil {
		return br.err
	}
	if br.header != nil {
		return errors.New("xz: header already set")
	}
	br.header = hdr
	br.headerLen = hdrLen

	br.cxz = countingReader{r: br.xz}

	var err error
	br.fr, err = br.cfg.newFilterReader(&br.cxz, hdr.filters)
	if err != nil {
		br.err = err
		return err
	}
	if br.hash.Size() != 0 {
		br.r = io.TeeReader(br.fr, br.hash)
	} else {
		br.r = br.fr
	}

	return nil
}

// unpaddedSize computes the unpadded size for the block.
func (br *blockReader) unpaddedSize() int64 {
	n := int64(br.headerLen)
	n += br.cxz.n
	n += int64(br.hash.Size())
	return n
}

// record returns the index record for the current block.
func (br *blockReader) record() record {
	return record{br.unpaddedSize(), br.uncompressed}
}

var errUnexpectedEndOfBlock = errors.New("xz: unexpected end of block")

// Read reads data from the block.
func (br *blockReader) Read(p []byte) (n int, err error) {
	if br.err != nil {
		return 0, br.err
	}

	if br.header == nil {
		hdr, hdrLen, err := readBlockHeader(br.xz)
		if err != nil {
			br.err = err
			return 0, err
		}
		if err = br.setHeader(hdr, hdrLen); err != nil {
			br.err = err
			return 0, err
		}
	}

	n, err = br.r.Read(p)
	br.uncompressed += int64(n)

	u := br.header.uncompressedSize
	if u >= 0 && br.uncompressed > u {
		br.err = errors.New("xz: wrong uncompressed size for block")
		return n, br.err
	}
	c := br.header.compressedSize
	if c >= 0 && br.cxz.n > c {
		br.err = errors.New("xz: wrong compressed size for block")
		return n, br.err
	}
	if err != io.EOF {
		if err != nil {
			br.err = err
		}
		return n, err
	}

	// EOF of the LZMA stream
	if br.uncompressed < u || br.cxz.n < c {
		br.err = errUnexpectedEndOfBlock
		return n, br.err
	}

	s := br.hash.Size()
	k := padLen(br.cxz.n)
	q := make([]byte, k+s, k+2*s)
	if _, err = io.ReadFull(br.cxz.r, q); err != nil {
		if err == io.EOF {
			err = io.ErrUnexpectedEOF
		}
		br.err = err
		return n, err
	}
	if !allZeros(q[:k]) {
		br.err = errors.New("xz: non-zero block padding")
		return n, br.err
	}
	checkSum := q[k:]
	computedSum := br.hash.Sum(checkSum[s:])
	if !bytes.Equal(checkSum, computedSum) {
		br.err = errors.New("xz: checksum error for block")
		return n, br.err
	}

	br.err = io.EOF
	return n, io.EOF
}

// Close closes the block reader and the LZMA2 reader supporting it.
func (br *blockReader) Close() error {
	if br.err == errReaderClosed {
		return errReaderClosed
	}
	if br.fr != nil {
		if err := br.fr.Close(); err != nil {
			br.err = err
			return err
		}
	}
	br.err = errReaderClosed
	return nil
}

// stReader provides the single-threaded stream reader.
type stReader struct {
	cfg *ReaderConfig
	xz  io.Reader

	br    blockReader
	index []record
	flags byte

	err error
}

// newSingleThreadStreamReader provides a streamReader. Note that it requires
// the header before Read can be called.
func newSingleThreadStreamReader(xz io.Reader, cfg *ReaderConfig) streamReader {
	return &stReader{cfg: cfg, xz: xz}
}

// reset provides the header information for the stream reader.
func (sr *stReader) reset(hdr *header) error {
	h, err := newHash(hdr.flags)
	if err != nil {
		return err
	}
	*sr = stReader{
		cfg:   sr.cfg,
		xz:    sr.xz,
		flags: hdr.flags,
	}
	sr.br.init(sr.xz, sr.cfg, h)
	return nil
}

// Read reads the uncompressed data from the stream reader. Note that the header
// must be set before it can be used.
func (sr *stReader) Read(p []byte) (n int, err error) {
	if sr.err != nil {
		return 0, sr.err
	}
	for n < len(p) {
		k, err := sr.br.Read(p[n:])
		n += k
		if err != nil {
			if err == io.EOF {
				sr.index = append(sr.index, sr.br.record())
				if err = sr.br.Close(); err != nil {
					sr.err = err
					return n, err
				}
				sr.br.reset()
				continue
			}
			if err == errIndexIndicator {
				err = readTail(sr.xz, sr.index, sr.flags)
				if err != nil {
					sr.err = err
					return n, err
				}
				err = io.EOF
			}
			sr.err = err
			return n, err
		}
	}

	return n, nil
}

// Close closes the single-threaded stream reader and closes the block reader.
func (sr *stReader) Close() error {
	if sr.err == errReaderClosed {
		return errReaderClosed
	}
	if err := sr.br.Close(); err != nil {
		sr.err = err
		return err
	}
	sr.err = errReaderClosed
	return nil
}

// readHeader reads header from the reader and skips padding if the padding
// argument is true. A possible outcome is io. EOF. If there is a problem with
// the padding errPadding is returned.
func readHeader(r io.Reader, padding bool) (hdr *header, err error) {
	p := make([]byte, HeaderLen)
	if padding {
	loop:
		for {
			n, err := io.ReadFull(r, p)
			if err != nil {
				if err == io.ErrUnexpectedEOF {
					if allZeros(p[:n]) {
						if n%4 != 0 {
							return nil, errPadding
						}
						return nil, io.EOF
					}
				}
				return nil, err
			}
			for i, b := range p {
				if b != 0 {
					if i == 0 {
						break loop
					}
					if i%4 != 0 {
						return nil, errPadding
					}
					n = copy(p, p[i:])
					_, err = io.ReadFull(r, p[n:])
					if err != nil {
						return nil, err
					}
					break loop
				}
			}
		}
	} else {
		_, err = io.ReadFull(r, p)
		if err != nil {
			return nil, err
		}
	}
	hdr = new(header)
	if err = hdr.UnmarshalBinary(p); err != nil {
		return nil, err
	}
	return hdr, nil
}

// readTail reads the index body and the xz footer.
func readTail(xz io.Reader, rindex []record, flags byte) error {
	index, n, err := readIndexBody(xz, len(rindex))
	if err != nil {
		if err == io.EOF {
			err = io.ErrUnexpectedEOF
		}
		return err
	}

	for i, rec := range index {
		if rec != rindex[i] {
			return fmt.Errorf("xz: record %d is %v; want %v",
				i, rec, rindex[i])
		}
	}

	p := make([]byte, footerLen)
	if _, err = io.ReadFull(xz, p); err != nil {
		if err == io.EOF {
			err = io.ErrUnexpectedEOF
		}
		return err
	}
	var f footer
	if err = f.UnmarshalBinary(p); err != nil {
		return err
	}
	if f.flags != flags {
		return errors.New("xz: footer flags incorrect")
	}
	if f.indexSize != int64(n)+1 {
		return errors.New("xz: index size in footer wrong")
	}
	return nil
}

// mtReader supports the multi-threaded reading of LZMA streams.
type mtReader struct {
	cfg *ReaderConfig
	xz  io.Reader

	ctx    context.Context
	cancel context.CancelFunc

	errCh    chan error
	streamCh chan mtrStreamTask
	workCh   chan mtrWorkerTask

	index []record
	flags byte

	br     blReader
	doneCh chan struct{}

	err error
}

// blReader provides an abstract interface for a block reader.
type blReader interface {
	io.ReadCloser
	record() record
}

// blr transports block reader information including a channel that must be
// closed if not nil after the block has been completely read.
type blr struct {
	r    blReader
	done chan struct{}
}

// mtrStreamTask is a channel that provides a single blr value.
type mtrStreamTask struct {
	blrCh <-chan blr
}

// mtrWorkerTask provides the data of single block to the worker. It must
// decompress the data and provide a virtual block reader to the blr channel. A
// done channel will not be required.
type mtrWorkerTask struct {
	hdr    *blockHeader
	hdrLen int
	data   []byte
	blrCh  chan<- blr
}

// newMultiThreadStreamReader creates multithreaded reader. Note that reset with
// a header must be called before Read can be used.
func newMultiThreadStreamReader(xz io.Reader, cfg *ReaderConfig) streamReader {
	return &mtReader{xz: xz, cfg: cfg}
}

// reset provides the header information to the multi-threaded stream reader. It
// starts the stream goroutine.
func (sr *mtReader) reset(hdr *header) error {
	if sr.cancel != nil {
		sr.cancel()
	}
	ctx, cancel := context.WithCancel(context.Background())
	*sr = mtReader{
		xz:       sr.xz,
		cfg:      sr.cfg,
		ctx:      ctx,
		cancel:   cancel,
		errCh:    make(chan error, 1),
		streamCh: make(chan mtrStreamTask, sr.cfg.Workers),
		workCh:   make(chan mtrWorkerTask, sr.cfg.Workers),
		flags:    hdr.flags,
	}
	go mtrStream(ctx, sr.xz, sr.cfg, sr.flags, sr.streamCh, sr.workCh,
		sr.errCh)
	return nil
}

// Read reads the data from the multi-threaded stream reader.
func (sr *mtReader) Read(p []byte) (n int, err error) {
	if sr.err != nil {
		return 0, sr.err
	}

	handle := func(err error) (int, error) {
		if err == io.EOF {
			err = io.ErrUnexpectedEOF
		}
		sr.err = err
		sr.cancel()
		if sr.br != nil {
			sr.br.Close()
			sr.br = nil
		}
		return n, err
	}

	for n < len(p) {
		if sr.br == nil {
			var (
				s  mtrStreamTask
				ok bool
			)
			select {
			case s, ok = <-sr.streamCh:
				if !ok {
					err = readTail(sr.xz, sr.index, sr.flags)
					if err != nil {
						return handle(err)
					}
					sr.err = io.EOF
					sr.cancel()
					return n, io.EOF
				}
			case err = <-sr.errCh:
				return handle(err)
			}
			select {
			case blr := <-s.blrCh:
				sr.br = blr.r
				sr.doneCh = blr.done
			case err = <-sr.errCh:
				return handle(err)
			}
		}
		k, err := sr.br.Read(p[n:])
		n += k
		if err != nil {
			if cerr := sr.br.Close(); cerr != nil {
				return handle(cerr)
			}
			if err == io.EOF {
				sr.index = append(sr.index, sr.br.record())
				sr.br = nil
				if sr.doneCh != nil {
					close(sr.doneCh)
				}
				continue
			}
			sr.br = nil
			return handle(err)
		}
	}

	return n, nil
}

// Close closes the multi-threaded stream reader and has to be called to cancel
// all goroutines that have been started.
func (sr *mtReader) Close() error {
	if sr.err == errReaderClosed {
		return sr.err
	}
	if sr.br != nil {
		sr.br.Close()
	}
	sr.cancel()
	sr.err = errReaderClosed
	return nil
}

// mtrStream provides the go routine that creates the work for the
// multi-threaded readers. It also supports blocks that cannot be read in
// parallel, because they are not providing the compressed size.
func mtrStream(ctx context.Context, xz io.Reader, cfg *ReaderConfig, flags byte,
	streamCh chan<- mtrStreamTask, workCh chan mtrWorkerTask,
	errCh chan<- error) {

	send := func(err error) (stop bool) {
		select {
		case errCh <- err:
			return false
		case <-ctx.Done():
			return true
		}
	}

	hh, err := newHash(flags)
	if err != nil {
		send(err)
		return
	}
	checkSize := int64(hh.Size())
	workers := 0
	for {
		hdr, hdrLen, err := readBlockHeader(xz)
		if err != nil {
			if err == errIndexIndicator {
				close(streamCh)
				return
			}
			send(err)
			return
		}
		blrCh := make(chan blr, 1)
		s := mtrStreamTask{blrCh: blrCh}
		if hdr.compressedSize < 0 { // block without compressed size
			// We need to setup a new block reader.
			h, err := newHash(flags)
			if err != nil {
				send(err)
				return
			}
			br := new(blockReader)
			br.init(xz, cfg, h)
			if err = br.setHeader(hdr, hdrLen); err != nil {
				send(err)
				return
			}
			// We will have to wait that the block reader has been
			// processed by the Read function. The doneCh will get
			// this signal.
			doneCh := make(chan struct{})
			select {
			case <-ctx.Done():
				return
			case blrCh <- blr{r: br, done: doneCh}:
			}
			// We are sending the task s to the stream channel.
			select {
			case <-ctx.Done():
				return
			case streamCh <- s:
			}
			// We are waiting until the task is completed.
			select {
			case <-ctx.Done():
				return
			case <-doneCh:
			}
			continue
		}
		// We are creating a task for the worker.
		w := mtrWorkerTask{
			hdr:    hdr,
			hdrLen: hdrLen,
			data: make([]byte, hdr.compressedSize+
				int64(padLen(hdr.compressedSize))+checkSize),
			blrCh: blrCh,
		}
		// We are reading the data for the worker.
		if _, err = io.ReadFull(xz, w.data); err != nil {
			send(err)
			return
		}
		// We are sending the stream task.
		select {
		case <-ctx.Done():
			return
		case streamCh <- s:
		}
		if workers < cfg.Workers {
			go mtrWork(ctx, cfg, flags, workCh, errCh)
			workers++
		}
		// We are sending the work tasks.
		select {
		case <-ctx.Done():
			return
		case workCh <- w:
		}
	}
}

// blockResultReader provides a virtual block reader.
type blockResultReader struct {
	*bytes.Buffer
	rec record
}

// Close for the block result reader does nothing.
func (r *blockResultReader) Close() error { return nil }

// record returns the record for the reader.
func (r *blockResultReader) record() record { return r.rec }

// mtrWork is the worker go routine for the multi-threaded stream reader.
func mtrWork(ctx context.Context, cfg *ReaderConfig, flag byte,
	workCh <-chan mtrWorkerTask, errCh chan<- error) {
	send := func(err error) (stop bool) {
		select {
		case errCh <- err:
			return false
		case <-ctx.Done():
			return true
		}
	}

	var br blockReader
	defer br.Close()
	hash, err := newHash(flag)
	if err != nil {
		send(err)
		return
	}

	for {
		var w mtrWorkerTask
		select {
		case <-ctx.Done():
			return
		case w = <-workCh:
		}
		br.init(bytes.NewReader(w.data), cfg, hash)
		if err = br.setHeader(w.hdr, w.hdrLen); err != nil {
			send(err)
			return
		}
		buf := new(bytes.Buffer)
		if w.hdr.uncompressedSize >= 0 {
			buf.Grow(int(w.hdr.uncompressedSize))
		}
		if _, err = io.Copy(buf, &br); err != nil {
			send(err)
			return
		}
		if err = br.Close(); err != nil {
			send(err)
			return
		}
		select {
		case <-ctx.Done():
			return
		case w.blrCh <- blr{
			r: &blockResultReader{Buffer: buf, rec: br.record()}}:
		}
	}
}
