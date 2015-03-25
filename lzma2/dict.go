package lzma2

// Maximum and minimum supported dictionary size.
const (
	MinDictSize = 1 << 12
	MaxDictSize = 1<<32 - 1
)

// errDist indicates that the distance is out of range.
var errDist = newError("distance out of range")

// readerDict represents the dictionary for reading. It is a ring buffer using
// the end field as head for the dictionary.
type readerDict struct {
	buffer
	bufferSize int
}

// newReaderDict creates a new reader dictionary. The capacity of the ring
// buffer will be the maximum of historySize and bufferSize.
func newReaderDict(historySize, bufferSize int) (rd *readerDict, err error) {
	if !(1 <= historySize && int64(historySize) < MaxDictSize) {
		return nil, newError("historySize out of range")
	}
	if bufferSize <= 0 {
		return nil, newError("bufferSize must be greater than zero")
	}
	capacity := historySize
	if bufferSize > capacity {
		capacity = bufferSize
	}
	rd = &readerDict{bufferSize: bufferSize}
	err = initBuffer(&rd.buffer, capacity)
	return
}

// Offset returns the offset of the dictionary head.
func (rd *readerDict) Offset() int64 {
	return rd.end
}

// WriteRep writes a repetition with the given distance. While distance is
// given here as int64 the actual limit is the maximum of the int type.
func (rd *readerDict) WriteRep(dist int64, n int) (written int, err error) {
	if !(1 <= dist && dist <= int64(rd.Len())) {
		return 0, errDist
	}
	return rd.WriteRepOff(n, rd.end-dist)
}

// Byte returns a byte at the given distance.
func (rd *readerDict) Byte(dist int) byte {
	c, _ := rd.ReadByteAt(rd.end - int64(dist))
	return c
}

// writerDict is the dictionary used for writing. It is a ring buffer using the
// cursor offset for the dictionary head. The capacity for the buffer is
// the sum of historySize and bufferSize.
//
// The actual writer uses encoderDict, which is an extension of writerDict to
// support the finding of string sequences in the history.
type writerDict struct {
	buffer
	bufferSize int
}

// initWriterDict initializes a writer dictionary.
func initWriterDict(wd *writerDict, historySize, bufferSize int) error {
	if !(1 <= historySize && int64(historySize) < MaxDictSize) {
		return newError("historySize out of range")
	}
	if bufferSize <= 0 {
		return newError("bufferSize must be greater than zero")
	}
	capacity := historySize + bufferSize
	*wd = writerDict{bufferSize: bufferSize}
	err := initBuffer(&wd.buffer, capacity)
	if err != nil {
		return err
	}
	wd.writeLimit = bufferSize
	return nil
}

// newWriterDict creates a new writer dictionary.
func newWriterDict(historySize, bufferSize int) (wd *writerDict, err error) {
	wd = new(writerDict)
	err = initWriterDict(wd, historySize, bufferSize)
	return
}

// HistorySize returns the history length.
func (wd *writerDict) HistorySize() int {
	return wd.Cap() - wd.bufferSize
}

// Returns the byte at the given distance to the dictionary head.
func (wd *writerDict) Byte(dist int) byte {
	c, _ := wd.ReadByteAt(wd.cursor - int64(dist))
	return c
}

// Offset returns the offset of the head.
func (wd *writerDict) Offset() int64 {
	return wd.cursor
}

func (wd *writerDict) PeekHead(p []byte) (n int, err error) {
	return wd.ReadAt(p, wd.cursor)
}
