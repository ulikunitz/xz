package lzbase

import "io"

// errDist indicates that the distance is out of range.
var errDist = newError("distance out of range")

// ReaderDict represents the dictionary for reading. It is a ring buffer using
// the end field as head for the dictionary.
type ReaderDict struct {
	buffer
	bufferSize int64
}

// NewReaderDict creates a new reader dictionary. The capacity of the ring
// buffer will be the maximum of historySize and bufferSize.
func NewReaderDict(historySize, bufferSize int64) (rd *ReaderDict, err error) {
	if !(1 <= historySize && historySize < MaxDictSize) {
		return nil, newError("historySize out of range")
	}
	if bufferSize <= 0 {
		return nil, newError("bufferSize must be greater than zero")
	}
	capacity := historySize
	if bufferSize > capacity {
		capacity = bufferSize
	}
	rd = &ReaderDict{bufferSize: bufferSize}
	err = initBuffer(&rd.buffer, capacity)
	return
}

// offset returns the offset of the dictionary head.
func (rd *ReaderDict) offset() int64 {
	return rd.end
}

// writeRep writes a repetition with the given distance. While distance is
// given here as int64 the actual limit is the maximum of the int type.
func (rd *ReaderDict) writeRep(dist int64, n int) (written int, err error) {
	if !(1 <= dist && dist <= int64(rd.length())) {
		return 0, errDist
	}
	return rd.writeRepOff(n, rd.end-dist)
}

// byteAt returns a byte at the given distance.
func (rd *ReaderDict) byteAt(dist int64) byte {
	c, _ := rd.readByteAt(rd.end - dist)
	return c
}

// WriterDict is the dictionary used for writing. It includes also a hashtable.
// It is a ring buffer using the cursor offset for the dictionary head. The
// capacity for the buffer is the sum of historySize and bufferSize.
type WriterDict struct {
	buffer
	bufferSize int64
	t4         *hashTable
}

func NewWriterDict(historySize, bufferSize int64) (wd *WriterDict, err error) {
	if !(1 <= historySize && historySize < MaxDictSize) {
		return nil, newError("historySize out of range")
	}
	if bufferSize <= 0 {
		return nil, newError("bufferSize must be greater than zero")
	}
	capacity := historySize + bufferSize
	wd = &WriterDict{bufferSize: bufferSize}
	if err = initBuffer(&wd.buffer, capacity); err != nil {
		return nil, err
	}
	wd.writeLimit = bufferSize
	if wd.t4, err = newHashTable(historySize, 4); err != nil {
		return nil, err
	}
	return wd, nil
}

// historySize returns the history length.
func (wd *WriterDict) historySize() int64 {
	return wd.capacity() - wd.bufferSize
}

// byteDist returns the byte at the given distance to the dictionary head.
func (wd *WriterDict) byteAt(dist int64) byte {
	c, _ := wd.readByteAt(wd.cursor - dist)
	return c
}

// offset returns the offset of the head.
func (wd *WriterDict) offset() int64 {
	return wd.cursor
}

// peekHead reads bytes from the Head without moving it.
func (wd *WriterDict) peekHead(p []byte) (n int, err error) {
	return wd.readAt(p, wd.cursor)
}

// DiscardOP advances the head of the dictionary and writes the respective
// bytes into the hash table.
func (wd *WriterDict) DiscardOp(op Operation) error {
	n, err := wd.copyTo(wd.t4, op.Len())
	if err != nil {
		return err
	}
	if n < op.Len() {
		return errAgain
	}
	return nil
}

// CopyChunk copies the last n bytes into the given writer.
func (wd *WriterDict) copyChunk(w io.Writer, n int) (copied int, err error) {
	if n <= 0 {
		if n == 0 {
			return 0, nil
		}
		return 0, newError("CopyChunk: argument n must be non-negative")
	}
	return wd.copyAt(w, n, wd.cursor-int64(n))
}

// potentialOffsets creates a list of potential offsets for matches.
func (wd *WriterDict) potentialOffsets(p []byte, state *WriterState) []int64 {
	head := wd.offset()
	start := wd.start
	offs := make([]int64, 0, 32)
	// add potential offsets with highest priority at the top
	for i := 1; i < 11; i++ {
		// distance 1 to 8
		off := head - int64(i)
		if start <= off {
			offs = append(offs, off)
		}
	}
	if len(p) == 4 {
		// distances from the hash table
		offs = append(offs, wd.t4.Offsets(p)...)
	}
	for i := 3; i >= 0; i-- {
		// distances from the repetition for length less than 4
		dist := int64(state.rep[i]) + minDistance
		off := head - dist
		if start <= off {
			offs = append(offs, off)
		}
	}
	return offs
}

// errNoMatch indicates that no match could be found
var errNoMatch = newError("no match found")

// bestMatch finds the best match for the given offsets.
//
// TODO: compare all possible commands for compressed bits per encoded bits.
func (wd *WriterDict) bestMatch(offsets []int64, state *WriterState) (m match, err error) {
	// creates a match for 1
	head := wd.offset()
	off := int64(-1)
	length := 0
	for i := len(offsets) - 1; i >= 0; i-- {
		n := wd.equalBytes(head, offsets[i], MaxLength)
		if n > length {
			off, length = offsets[i], n
		}
	}
	if off < 0 {
		err = errNoMatch
		return
	}
	if length == 1 {
		dist := int64(state.rep[0]) + minDistance
		offRep0 := head - dist
		if off != offRep0 {
			err = errNoMatch
			return
		}
	}
	return match{distance: head - off, n: length}, nil
}

// errEmptyBuf indicates an empty buffer
var errEmptyBuf = newError("empty buffer")

// FindOp finds an operation for the head of the dictionary.
func (wd *WriterDict) FindOp(state *WriterState) (op Operation, err error) {
	p := make([]byte, 4)
	n, err := wd.peekHead(p)
	if err != nil && err != errAgain && err != io.EOF {
		return nil, err
	}
	if n <= 0 {
		if n < 0 {
			panic("strange n")
		}
		return nil, errEmptyBuf
	}
	offs := wd.potentialOffsets(p[:n], state)
	m, err := wd.bestMatch(offs, state)
	if err == errNoMatch {
		return lit{b: p[0]}, nil
	}
	if err != nil {
		return nil, err
	}
	return m, nil
}
