package newlzma

import (
	"errors"
	"fmt"
	"io"
)

// matcher is an interface that allows the identification of potential
// matches for words with a constant length greater or equal 2.
//
// The absolute positions of potential matches are provided by the
// Matches function.
type matcher interface {
	io.Writer
	WordLen() int
	Pos() int64
	Matches(word []byte) (positions []int64, err error)
}

type encoderBuffer struct {
	buffer
	matcher
}

func (b *encoderBuffer) Write(p []byte) (n int, err error) {
	n, err = b.buffer.Write(p)
	k, merr := b.matcher.Write(p[:n])
	if merr != nil {
		panic(fmt.Errorf("matcher wrote %d of %d bytes because of %s",
			k, n, merr))
	}
	return
}

func (b *encoderBuffer) ReadByteAt(pos int64) (c byte, err error) {
	d := b.Pos() - pos
	if !(0 < d && d <= int64(b.Buffered())) {
		return 0, errors.New("ReadByteAt: position not buffered")
	}
	i := b.front - int(d)
	if i < 0 {
		i += len(b.data)
	}
	return b.data[i], nil
}

func (b *encoderBuffer) ReadAt(p []byte, pos int64) (n int, err error) {
	d := b.Pos() - pos
	if !(0 < d && d <= int64(b.Buffered())) {
		return 0, errors.New("ReadAt: position outside buffer")
	}
	n = int(d)
	if n < len(p) {
		p = p[:n]
		err = errors.New("ReadAt: insufficient data in buffer")
	}
	i := b.front - n
	if i < 0 {
		i += len(b.data)
	}
	k := copy(p, b.data[i:])
	if k < n {
		copy(p[k:], b.data)
	}
	return
}
