package lzma

var (
	errDist = newError("distance out of range")
)

type readerDict struct {
	buffer
	bufferLen int
}

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

func (rd *readerDict) Offset() int64 {
	return rd.end
}

func (rd *readerDict) WriteRep(dist int64, n int) (written int, err error) {
	if !(1 <= dist && dist <= int64(rd.Len())) {
		return 0, errDist
	}
	return rd.WriteRepOff(n, rd.end-dist)
}

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
