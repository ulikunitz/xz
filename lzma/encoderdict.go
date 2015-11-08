package lzma

import (
	"errors"
	"io"
)

// writerDict provides a writer dictionary that doesn't support a
// matcher. It is used for writing the operations out after they have
// been identified. Note that the buffer includes the dictionary and the
// write buffer so its size must be the sum of dictionary capacity and
// buffer size.
type writerDict struct {
	buf      buffer
	head     int64
	capacity int
}

// initWriterDict initializes the writer dictionary. The bufSize
// argument provides the size of the additional write buffer.
func initWriterDict(d *writerDict, dictCap, bufSize int) error {
	if !(1 <= dictCap && dictCap <= maxDictCap) {
		return errors.New("dictionary capacity out of range")
	}
	if bufSize < 1 {
		return errors.New(
			"writer dictionary: buffer size must be larger " +
				"than zero")
	}
	*d = writerDict{capacity: dictCap}
	return initBuffer(&d.buf, dictCap+bufSize)
}

// Reset puts the current position to 0.
func (d *writerDict) Reset() {
	d.head = 0
}

// Buffered gives the number of bytes available for a following Read or
// Advance.
func (d *EncoderDict) Buffered() int {
	return d.buf.Buffered()
}

// Available returns the number of bytes that can be written by a
// following Write call.
func (d *writerDict) Available() int {
	return d.buf.Available() - d.DictLen()
}

// DictCap returns the dictionary capacity.
func (d *writerDict) DictCap() int {
	return d.capacity
}

// DictLen returns the current number of bytes of the dictionary. The
// number has dictionary capacity as upper limit.
func (d *writerDict) DictLen() int {
	if int64(d.capacity) > d.head {
		return int(d.head)
	}
	return d.capacity
}

// Len returns the size of the data available. The return value might be
// larger than the dictionary capacity.
func (d *writerDict) Len() int {
	n := d.buf.Available()
	if int64(n) > d.head {
		return int(d.head)
	}
	return n
}

// Pos returns the current position of the dictionary head.
func (d *writerDict) Pos() int64 {
	return d.head
}

// ByteAt returns a byte from the dictionary. The distance is the
// positive difference from the current head. A distance of 1 will
// return the top-most byte in the dictionary.
func (d *writerDict) ByteAt(distance int) byte {
	if !(0 < distance && distance <= d.DictLen()) {
		return 0
	}
	i := d.buf.rear - distance
	if i < 0 {
		i += len(d.buf.data)
	}
	return d.buf.data[i]
}

// Write puts new data into the dictionary.
func (d *writerDict) Write(p []byte) (n int, err error) {
	n = len(p)
	m := d.Available()
	if n > m {
		p = p[:m]
		err = ErrNoSpace
	}
	var werr error
	n, werr = d.buf.Write(p)
	if werr != nil {
		err = werr
	}
	return n, err
}

// Read reads data from the buffer in front of the dictionary. Reading
// has the same effect as Advance on the dictionary.
func (d *writerDict) Read(p []byte) (n int, err error) {
	n, err = d.buf.Read(p)
	d.head += int64(n)
	return n, err
}

// Advance moves the dictionary head ahead by the given number of bytes.
func (d *writerDict) Advance(n int) (advanced int, err error) {
	advanced, err = d.buf.Discard(n)
	d.head += int64(advanced)
	return advanced, err
}

// CopyN copies the n topmost bytes of the dictionary. The maximum for n
// is given by the Len() method.
func (d *writerDict) CopyN(w io.Writer, n int) (written int, err error) {
	if n <= 0 {
		if n == 0 {
			return 0, nil
		}
		return 0, errors.New("CopyN: negative argument")
	}
	m := d.Len()
	if n > m {
		n = m
		err = ErrNoSpace
	}
	i := d.buf.rear - int(n)
	if i >= 0 {
		k, werr := w.Write(d.buf.data[i:d.buf.rear])
		if werr != nil {
			err = werr
		}
		return k, err
	}
	i += len(d.buf.data)
	k, werr := w.Write(d.buf.data[i:])
	written = k
	if werr != nil {
		return written, werr
	}
	k, werr = w.Write(d.buf.data[:d.buf.rear])
	written += k
	if werr != nil {
		err = werr
	}
	return written, err
}

// Literal returns the the byte at the position of the head. The method
// returns 0 if no bytes are buffered.
func (d *EncoderDict) Literal() byte {
	if d.buf.rear == d.buf.front {
		return 0
	}
	return d.buf.data[d.buf.rear]
}

// matcher is an interface that allows the identification of potential
// matches for words with a constant length greater or equal 2.
//
// The absolute offset of potential matches are provided by the
// Matches method. The current position of the matcher is provided by
// the Pos method.
//
// The Reset method clears the matcher completely but starts new data
// at the given position.
type matcher interface {
	io.Writer
	WordLen() int
	Matches(word []byte) (positions []int64)
	Reset()
}

// EncoderDict provides the dictionary for the encoder. It includes a
// matcher for searching matching strings in the dictionary.
// The type includes a write buffer in front of the actual dictionary.
// The actual size of the buffer is the total of dictionary capacity and
// the size of the buffer for writing.
type EncoderDict struct {
	writerDict
	m matcher
	p []byte
}

// NewEncoderDict creates a new encoder dictionary. The initial position
// and length of the dictionary will be zero. The argument dictCap
// provides the capacity of the dictionary. The argument bufSize gives
// the size of the write buffer.
func NewEncoderDict(dictCap, bufSize int) (d *EncoderDict, err error) {
	m, err := newHashTable(dictCap, 4)
	if err != nil {
		return nil, err
	}
	d = &EncoderDict{m: m}
	if err = initWriterDict(&d.writerDict, dictCap, bufSize); err != nil {
		return nil, err
	}
	d.p = make([]byte, maxMatchLen)
	return d, nil
}

// Reset clears the dictionary. After return the state of the dictionary
// is the same as after NewEncoderDict.
func (d *EncoderDict) Reset() {
	d.writerDict.Reset()
	d.m.Reset()
}

// Read reads data from the buffer in front of the dictionary. Reading
// has the same effect as Advance on the dictionary.
func (d *EncoderDict) Read(p []byte) (n int, err error) {
	n, err = d.buf.Read(p)
	d.head += int64(n)
	if _, werr := d.m.Write(p[:n]); werr != nil {
		err = werr
	}
	return n, err
}

// Advance moves the dictionary head ahead by the given number of bytes.
func (d *EncoderDict) Advance(n int) (advanced int, err error) {
	advanced, err = d.Read(d.p[:n])
	if advanced != n && err == nil {
		err = errors.New("Advance: short buffer")
	}
	return
}

// Matches returns potential distances for the word at the head of the
// dictionary. If there are not enough bytes a nil slice will be
// returned.
func (d *EncoderDict) Matches() (distances []int) {
	w := d.m.WordLen()
	if d.Buffered() < w {
		return nil
	}
	p := make([]byte, w)
	// Peek doesn't return errors and we have ensured that there are
	// enough bytes.
	d.buf.Peek(p)
	positions := d.m.Matches(p)
	n := int64(d.DictLen())
	for _, pos := range positions {
		d := d.head - pos
		if 0 < d && d <= n {
			distances = append(distances, int(d))
		}
	}
	return distances
}

// MatchLen computes the length of the match at the given distance with
// the bytes at the head of the dictionary. The function returns zero
// if no match is found.
func (d *EncoderDict) MatchLen(dist int) int {
	if !(0 < dist && dist <= d.DictLen()) {
		return 0
	}
	b := d.Buffered()
	return d.buf.EqualBytes(b+dist, b, maxMatchLen)
}
