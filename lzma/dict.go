package lzma

// errDist indicates that the distance is out of range.
var errDist = newError("distance out of range")

// readerDict represents the dictionary for reading. It is a ring buffer using
// the end field as head for the dictionary.
type readerDict struct {
	buffer
	bufferLen int
}

// newReaderDict creates a new reader dictionary. The capacity of the ring
// buffer will be the maximum of historyLen and bufferLen.
func newReaderDict(historyLen, bufferLen int) (rd *readerDict, err error) {
	if !(1 <= historyLen && int64(historyLen) < MaxDictLen) {
		return nil, newError("historyLen out of range")
	}
	if bufferLen <= 0 {
		return nil, newError("bufferLen must be greater than zero")
	}
	capacity := historyLen
	if bufferLen > capacity {
		capacity = bufferLen
	}
	rd = &readerDict{bufferLen: bufferLen}
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

type writerDict struct {
	buffer
}

func newWriterDict(historyLen, bufferLen int) (wd *writerDict, err error) {
	if !(1 <= historyLen && int64(historyLen) < MaxDictLen) {
		return nil, newError("historyLen out of range")
	}
	if bufferLen <= 0 {
		return nil, newError("bufferLen must be greater than zero")
	}
	capacity := historyLen + bufferLen
	wd = &writerDict{}
	err = initBuffer(&wd.buffer, capacity)
	if err != nil {
		return nil, err
	}
	wd.writeLimit = bufferLen
	return wd, nil
}

func (wd *writerDict) Byte(dist int) byte {
	panic("TODO")
}

func (wd *writerDict) Offset() int64 {
	return wd.cursor
}
