package lzma

type finder struct {
	t4 *hashTable
	t2 *hashTable
}

func initFinder(f *finder, historyLen int) error {
	if historyLen < 1 {
		return newError("history length must be at least one byte")
	}
	if int64(historyLen) > MaxDictLen {
		return newError("history length must be less than 2^32")
	}
	panic("TODO")
}

type encoderDict struct {
	writerDict
	f finder
}

func newEncoderDict(historyLen, bufferLen int) (d *encoderDict, err error) {
	d = new(encoderDict)
	err = initWriterDict(&d.writerDict, historyLen, bufferLen)
	if err != nil {
		return nil, err
	}
	panic("TODO")
}

var errEmptyBuf = newError("empty buffer")

func (d *encoderDict) ReadOp() (op operation, err error) {
	panic("TODO")
}
