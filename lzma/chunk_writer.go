package lzma

import (
	"bytes"
	"io"

	"github.com/ulikunitz/lz"
)

type chunkWriter struct {
	encoder
	blk lz.Block
	buf bytes.Buffer
	seq lz.Sequencer
	w   io.Writer
	// dirReset is true if reset has been done
	dirReset bool
	// spReset is true if spReset has been done
	spReset bool
	err     error
}

func (w *chunkWriter) init(z io.Writer, seq lz.Sequencer, data []byte,
	props Properties) error {
	*w = chunkWriter{
		seq:     seq,
		encoder: encoder{window: seq.WindowPtr()},
		blk: lz.Block{
			Sequences: w.blk.Sequences[:0],
			Literals:  w.blk.Literals[:0],
		},
		buf: w.buf,
		w:   z,
	}
	if err := w.window.Reset(data); err != nil {
		return err
	}
	w.state.init(props)
	return nil
}

const (
	maxChunkSize             = 1 << 16
	maxUncompressedChunkSize = 1 << 21
)

func updateBlock(blk *lz.Block, litIndex, seqIndex int) {
	n := copy(blk.Literals, blk.Literals[litIndex:])
	blk.Literals = blk.Literals[:n]
	n = copy(blk.Sequences, blk.Sequences[seqIndex:])
	blk.Sequences = blk.Sequences[:n]
}

func (w *chunkWriter) writeChunk() error {
	// prepare writing
	w.buf.Reset()
	w.re.init(&w.buf)
	n := 0
	var oldState state
	oldState.deepCopy(&w.state)

	// loop until enough uncompressed data is written or the output is too
	// long
	var err error
loop:
	for {
		var litIndex = 0
		for k, s := range w.blk.Sequences {
			i := litIndex
			litIndex += int(s.LitLen)
			for j, c := range w.blk.Literals[i:litIndex] {
				err = w.writeLiteral(c)
				if err != nil {
					return err
				}
				n++
				if n >= maxUncompressedChunkSize {
					w.blk.Sequences[k].LitLen -=
						uint32(j) + 1
					updateBlock(&w.blk, i+j+1, k)
					break loop
				}
			}

			// TODO: remove checks
			if s.Offset < minDistance {
				panic("s.Offset < minDistance")
			}
			if s.MatchLen < minMatchLen {
				panic("s.MatchLen < minMatchLen")
			}

			o, m := s.Offset-1, s.MatchLen
			for {
				var u uint32
				if m <= maxMatchLen {
					u = m
				} else if m >= maxMatchLen+minMatchLen {
					u = maxMatchLen
				} else {
					u = m - minMatchLen
				}
				if n+int(u) > maxUncompressedChunkSize {
					w.blk.Sequences[k].LitLen = 0
					updateBlock(&w.blk, litIndex, k)
					break loop
				}
				if err = w.writeMatch(o, u); err != nil {
					return err
				}
				n += int(u)
				m -= u
				if m == 0 {
					break
				}
			}
		}
		w.blk.Sequences = w.blk.Sequences[:0]
		for j, c := range w.blk.Literals[litIndex:] {
			if err = w.writeLiteral(c); err != nil {
				return err
			}
			n++
			if n >= maxUncompressedChunkSize {
				updateBlock(&w.blk, litIndex+j+1,
					len(w.blk.Sequences))
				break loop
			}
		}

		_, err := w.seq.Sequence(&w.blk, 0)
		if err != nil {
			if err == lz.ErrEmptyBuffer {
				w.blk.Literals = w.blk.Literals[:0]
				w.blk.Sequences = w.blk.Sequences[:0]
				break loop
			}
			return err
		}
	}

	if err = w.re.Close(); err != nil {
		return err
	}

	headerLen := 5
	if !w.spReset {
		headerLen += 1
	}
	k := w.buf.Len()
	h := chunkHeader{size: n}
	m := 3 + n
	if m < headerLen+k {
		w.state.deepCopy(&oldState)
		// uncompressed write
		if w.dirReset {
			h.control = cU
		} else {
			h.control = cUD
			w.dirReset = true
		}
		// TODO: write header

		// TODO: use the buffer array?
		p := w.buf.Bytes()
		if cap(p) < m {
			p = make([]byte, m)
		} else {
			p = p[:3+n]
		}
		_, err := h.append(p[:0])
		if err != nil {
			return err
		}

		k, err := w.window.ReadAt(p[3:], w.encoder.pos-int64(n))
		if err != nil {
			return err
		}
		if k != n {
			panic("k != n")
		}

		if _, err = w.w.Write(p); err != nil {
			return err
		}

		return nil
	}

	// compressed write
	h.compressedSize = k
	if !w.spReset {
		h.properties = w.state.Properties
		if !w.dirReset {
			h.control = cCSPD
			w.dirReset = true
		} else {
			h.control = cCSP
		}
		w.spReset = true
	} else {
		h.control = cC
	}

	var a [6]byte
	p, err := h.append(a[:0])
	if err != nil {
		return err
	}

	if _, err = w.w.Write(p); err != nil {
		return err
	}
	if _, err = w.w.Write(w.buf.Bytes()); err != nil {
		return err
	}

	return nil
}

func (w *chunkWriter) Write(p []byte) (n int, err error) {
	if w.err != nil {
		return 0, w.err
	}
	for {
		var k int
		k, err = w.window.Write(p[n:])
		n += k
		if err == nil {
			return n, nil
		}
		if err != lz.ErrFullBuffer {
			w.err = err
			return n, err
		}
		if err = w.writeChunk(); err != nil {
			w.err = err
			return n, err
		}
	}
}

func (w *chunkWriter) flush() error {
	if w.err != nil {
		return w.err
	}
	for {
		if len(w.blk.Sequences) == 0 &&
			len(w.blk.Literals) == 0 &&
			w.window.Buffered() == 0 {
			return nil
		}
		if err := w.writeChunk(); err != nil {
			w.err = err
			return err
		}
	}
}

func (w *chunkWriter) Close() error {
	if err := w.flush(); err != nil {
		return err
	}
	w.err = errClosed
	return nil
}
