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
