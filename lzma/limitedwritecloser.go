package lzma

import (
	"errors"
	"fmt"
	"io"
)

type limitedWriteCloser struct {
	W      io.WriteCloser
	N      int64
	Closed bool
}

const minInt64 = -1 << 63

var errEarlyClose = errors.New("close called before limit reached")

func (lw *limitedWriteCloser) Write(p []byte) (n int, err error) {
	if lw.Closed {
		return 0, errClosed
	}
	if lw.N <= 0 {
		return 0, errLimit
	}
	if int64(len(p)) > lw.N {
		p = p[0:lw.N]
		err = errLimit
	}
	var werr error
	if n, werr = lw.W.Write(p); werr != nil {
		return n, werr
	}
	return n, err
}

func (lw *limitedWriteCloser) Close() error {
	if lw.Closed {
		return errClosed
	}
	if lw.N < 0 {
		panic(fmt.Errorf("lw.N has unexpected value %d", lw.N))
	}
	if lw.N > 0 {
		return errEarlyClose
	}
	if err := lw.W.Close(); err != nil {
		return err
	}
	lw.Closed = true
	return nil
}
