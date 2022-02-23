package lzma

import (
	"bytes"
	"io"

	"github.com/ulikunitz/lz"
)

const (
	maxChunkSize             = 1 << 16
	maxUncompressedChunkSize = 1 << 21
)

// chunkWriter is a writer that creates a series of LZMA2 chunks.
type chunkWriter struct {
	encoder
	blk      lz.Block
	buf      bytes.Buffer
	oldState state
	seq      lz.Sequencer
	w        io.Writer
	err      error
	// start position of the current chunk
	start int64
	// dirReset is true if reset has been done
	dirReset bool
	// spReset is true if spReset has been done
	spReset bool
}

// init initializes the chunkWriter. A set of initial data can be provided
// directly. The array is directly used by the Window.
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
	w.startChunk()
	return nil
}

// writeSequences writes sequences to the encoder until the limits for the chunk
// are reached or an error occurs.
func (w *chunkWriter) writeSequences() error {
	var err error
	max := w.start + maxUncompressedChunkSize
loop:
	for {
		litIndex := 0
		for k, s := range w.blk.Sequences {
			i := litIndex
			litIndex += int(s.LitLen)
			for j, c := range w.blk.Literals[i:litIndex] {
				if w.buf.Len()+w.re.cacheLen > maxChunkSize-8 ||
					w.pos >= max {
					w.blk.Sequences[k].LitLen -= uint32(j)
					updateBlock(&w.blk, i+j, k)
					break loop
				}
				err = w.writeLiteral(c)
				if err != nil {
					return err
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
				if w.pos+int64(u) > max ||
					w.buf.Len()+w.re.cacheLen >
						maxChunkSize-16 {
					w.blk.Sequences[k].LitLen = 0
					updateBlock(&w.blk, litIndex, k)
					break loop
				}
				if err = w.writeMatch(o, u); err != nil {
					return err
				}
				m -= u
				if m == 0 {
					break
				}
			}
		}
		w.blk.Sequences = w.blk.Sequences[:0]
		for j, c := range w.blk.Literals[litIndex:] {
			if w.buf.Len()+w.re.cacheLen > maxChunkSize-8 ||
				w.pos >= max {
				updateBlock(&w.blk, litIndex+j,
					len(w.blk.Sequences))
				break loop
			}
			if err = w.writeLiteral(c); err != nil {
				return err
			}
		}

		_, err := w.seq.Sequence(&w.blk, 0)
		if err != nil {
			if err == lz.ErrEmptyBuffer {
				w.blk.Literals = w.blk.Literals[:0]
				w.blk.Sequences = w.blk.Sequences[:0]
				return err
			}
			return err
		}

	}

	return nil
}

// clearBuffer consumes all data provided and writes then in a sequence of
// chunks. The last chunk will not be written out. Use the method finishChunnk
// for it.
func (w *chunkWriter) clearBuffer() error {
	var err error
	for {
		err = w.writeSequences()
		if err != nil {
			if err == lz.ErrEmptyBuffer {
				return nil
			}
			return err
		}
		if err = w.finishChunk(); err != nil {
			return err
		}
	}
}

// updateBlock copies remaining sequences and literals to the front of the
// slices in the block.
func updateBlock(blk *lz.Block, litIndex, seqIndex int) {
	n := copy(blk.Literals, blk.Literals[litIndex:])
	blk.Literals = blk.Literals[:n]
	n = copy(blk.Sequences, blk.Sequences[seqIndex:])
	blk.Sequences = blk.Sequences[:n]
}

// startChunk starts a new chunk.
func (w *chunkWriter) startChunk() {
	w.start = w.encoder.pos
	w.buf.Reset()
	w.re.init(&w.buf)
	w.oldState.deepCopy(&w.state)
}

// finishChunk writes a chunk out if there has been data written into the
// encoder.
func (w *chunkWriter) finishChunk() error {
	n := int(w.encoder.pos - w.start)
	if n == 0 {
		// no data, no chunk need to be written
		return nil
	}

	if err := w.re.Close(); err != nil {
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
		w.state.deepCopy(&w.oldState)
		// uncompressed write
		if w.dirReset {
			h.control = cU
		} else {
			h.control = cUD
			w.dirReset = true
		}

		p := w.buf.Bytes()
		if cap(p) < m {
			p = make([]byte, m)
		} else {
			p = p[:m]
		}
		_, err := h.append(p[:0])
		if err != nil {
			return err
		}

		k, err := w.window.ReadAt(p[3:], w.start)
		if err != nil {
			return err
		}
		if k != n {
			panic("k != n")
		}

		if _, err = w.w.Write(p); err != nil {
			return err
		}

		w.startChunk()
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

	w.startChunk()
	return nil

}

// Write writes data into the window until it is filled at which time all the
// buffer will be cleared and multiple chunks might be written.
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
		if err = w.clearBuffer(); err != nil {
			w.err = err
			return n, err
		}
		w.seq.Shrink()
	}
}

// Flush writes all buffered data to the underlying writer.
func (w *chunkWriter) Flush() error {
	if w.err != nil {
		return w.err
	}
	var err error
	if err = w.clearBuffer(); err != nil {
		w.err = err
		return err
	}
	if err = w.finishChunk(); err != nil {
		w.err = err
		return err
	}
	return nil
}

// Close writes all data into the underlying writer and adds an End-of-Stream
// Chunk. No further data can be added to the writer.
func (w *chunkWriter) Close() error {
	var err error
	if err = w.Flush(); err != nil {
		return err
	}
	// The EOS chunk is a single zero byte.
	var a [1]byte
	if _, err = w.w.Write(a[:]); err != nil {
		w.err = err
		return err
	}
	w.err = errClosed
	return nil
}
