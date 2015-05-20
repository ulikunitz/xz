package lzb

import (
	"errors"
	"fmt"
	"io"
)

// TODO
//
// - write fills buffer until full + compression is started at the very
//   end

type OpFinder interface {
	findOps(s *State, all bool) ([]operation, error)
	fmt.Stringer
}

// Writer produces an LZMA stream. EOS requests Close to write an
// end-of-stream marker.
type Writer struct {
	State    *State
	EOS      bool
	OpFinder OpFinder
	re       *rangeEncoder
	buf      *buffer
	closed   bool
}

func NewWriter(pw io.Writer, p Params) (w *Writer, err error) {
	buf, err := newBuffer(p.BufferSize + p.DictSize)
	if err != nil {
		return nil, err
	}
	d, err := newHashDict(buf, p.DictSize)
	if err != nil {
		return nil, err
	}
	d.sync()
	state := NewState(p.Properties, d)
	return NewWriterState(pw, state)
}

func NewWriterState(pw io.Writer, state *State) (w *Writer, err error) {
	if _, ok := state.dict.(*hashDict); !ok {
		return nil, errors.New(
			"state must support a writer (no hashDict)")
	}
	w = &Writer{
		State:    state,
		buf:      state.dict.buffer(),
		re:       newRangeEncoder(pw),
		OpFinder: Greedy,
	}
	return w, nil
}

// applyOps applies ops on the writer. The method assumes that all
// operations are correct and no conditions are violated.
func applyOps(ops []operation) error {
	panic("TODO")
}

func Write(p []byte) (n int, err error) {
	panic("TODO")
}

func Close() error {
	panic("TODO")
}
