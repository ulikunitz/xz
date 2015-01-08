package lzma

import "io"

// The index provides candidate offsets for a byte slice.
type index struct {
	t4 *hashTable
	t2 *hashTable
}

func initIndex(idx *index, historyLen int) error {
	if historyLen < 1 {
		return newError("history length must be at least one byte")
	}
	if int64(historyLen) > MaxDictLen {
		return newError("history length must be less than 2^32")
	}
	*idx = index{}
	var err error
	if idx.t4, err = newHashTable(historyLen, 4); err != nil {
		return err
	}
	if idx.t2, err = newHashTable(historyLen, 2); err != nil {
		return err
	}
	return nil
}

// Writes the slice into the index. It will never return an error.
func (idx *index) Write(p []byte) (n int, err error) {
	idx.t4.Write(p)
	idx.t2.Write(p)
	return len(p), nil
}

// The function search for potential matches for sizes 2 and 4.
func (idx *index) SearchMatches(p []byte) (offsets []int64) {
	switch len(p) {
	case 4:
		return idx.t4.Offsets(p)
	case 2:
		return idx.t2.Offsets(p)
	case 1:
		head := idx.t2.Offset()
		if head <= 0 {
			return nil
		}
		return []int64{head - 1}
	}
	return nil
}

type encoderDict struct {
	writerDict
	idx index
}

func newEncoderDict(historyLen, bufferLen int) (d *encoderDict, err error) {
	d = new(encoderDict)
	err = initWriterDict(&d.writerDict, historyLen, bufferLen)
	if err != nil {
		return nil, err
	}
	err = initIndex(&d.idx, historyLen)
	if err != nil {
		return nil, err
	}
	return
}

func (d *encoderDict) newMatch(off int64, n int) (m match, err error) {
	head := d.Offset()
	start := d.Offset() - int64(d.HistoryLen())
	if !(start <= off && off < head) {
		err = errOffset
		return
	}
	if !(minLength <= n && n <= maxLength) {
		err = newError("length out of range")
		return
	}
	dist := head - off
	return match{distance: dist, length: n}, nil
}

func (d *encoderDict) bestMatch(offsets []int64) (m match, err error) {
	head := d.Offset()
	off := int64(-1)
	n := 0
	for i := len(offsets) - 1; i >= 0; i-- {
		k := d.EqualBytes(head, offsets[i], maxLength)
		if k > n {
			off = offsets[i]
			n = k
		}
	}
	if off < 0 || n == 1 {
		err = errNoMatch
		return
	}
	return d.newMatch(off, n)
}

// TODO: code doesn't create "Short Rep Matches" and "Rep Matches" are not
// prioritized
func (d *encoderDict) findOp() (op operation, err error) {
	p := make([]byte, 4)
	n, err := d.PeekHead(p)
	if err != nil && err != errAgain && err != io.EOF {
		return nil, err
	}
	if n <= 0 {
		if n < 0 {
			panic("strange n")
		}
		return nil, errEmptyBuf
	}
	p = p[:n]
	for _, i := range []int{4, 2, 1} {
		if len(p) >= i {
			offs := d.idx.SearchMatches(p[:i])
			op, err = d.bestMatch(offs)
			if err == nil {
				return
			}
		}
	}
	return lit{p[0]}, nil
}
