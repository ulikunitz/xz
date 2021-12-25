package lzma

import (
	"errors"
	"io"

	"github.com/ulikunitz/lz"
)

type encoder struct {
	seq    lz.Sequencer
	w      *lz.Window
	pos    int64
	blk    lz.Block
	idxSeq int
	idxLit int
	state  state
	closed bool
	eos    bool
}

func (e *encoder) init(seq lz.Sequencer, p Properties) {
	e.blk.Sequences = e.blk.Sequences[:0]
	e.blk.Literals = e.blk.Literals[:0]
	*e = encoder{
		seq: seq,
		w:   seq.WindowPtr(),
		blk: e.blk,
	}
	e.state.init(p)
	// TODO
}

func (e *encoder) reset() {
	e.seq.Reset()
	e.blk.Sequences = e.blk.Sequences[:0]
	e.blk.Literals = e.blk.Literals[:0]
	e.pos = 0
	e.idxSeq = 0
	e.idxLit = 0
	e.state.reset()
	e.closed = false
	e.eos = false
}

var errClosed = errors.New("lzma: already closed")

// Write puts data into the sequencer window.
func (e *encoder) Write(p []byte) (n int, err error) {
	if e.closed {
		return 0, errClosed
	}
	return e.w.Write(p)
}

// ReadFrom puts data into the sequencer buffer.
func (e *encoder) ReadFrom(r io.Reader) (n int64, err error) {
	if e.closed {
		return 0, errClosed
	}
	return e.w.ReadFrom(r)
}

// Marks stream for providing an EOS marker when the end of the input stream is
// reached.
func (e *encoder) EOS() {
	e.eos = true
}

// Close closes the input stream.
func (e *encoder) Close() error {
	if e.closed {
		return errClosed
	}
	e.closed = true
	return nil
}

// ReadChunk reads a chunk from the encoded data. If err is io.EOF the file end
// has been reached. Note that we don't provide any facilities to reset the
// dictionary or the state of the encoding. We assume that all those things has
// happened before the encoder has been created.
func ReadChunk(p []byte) (n int, uncompressed bool, err error) {
	panic("TODO")
}

// Read reads the next bytes from the range encoder.
func Read(p []byte) (n int, err error) {
	panic("TODO")
}
