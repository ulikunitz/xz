package lzb

import (
	"errors"
	"io"
)

type greedyFinder struct{}

// Greedy provides a greedy operation finder.
var Greedy OpFinder

// don't want to expose the initialization of Greedy
func init() {
	Greedy = greedyFinder{}
}

// errNoMatch indicates that no match could be found
var errNoMatch = errors.New("no match found")

func bestMatch(d *hashDict, offsets []int64) (m match, err error) {
	off := int64(-1)
	length := 0
	for i := len(offsets) - 1; i >= 0; i-- {
		n := d.buf.equalBytes(d.head, offsets[i], MaxLength)
		if n >= length {
			off, length = offsets[i], n
		}
	}
	if off < 0 || length == 1 {
		err = errNoMatch
		return
	}
	return match{distance: d.head - off, n: length}, nil
}

var errEmptyBuf = errors.New("empty buffer")

func potentialOffsets(d *hashDict, p []byte) []int64 {
	start := d.start()
	offs := make([]int64, 0, 32)
	// add potential offsets with highest priority at the top
	for i := 1; i < 11; i++ {
		// distance 1 to 10
		off := d.head - int64(i)
		if start <= off {
			offs = append(offs, off)
		}
	}
	if len(p) == 4 {
		// distances from the hash table
		offs = append(offs, d.t4.Offsets(p)...)
	}
	return offs
}

func findOp(d *hashDict) (op operation, err error) {
	p := make([]byte, 4)
	n, err := d.buf.ReadAt(p, d.head)
	if err != nil && err != io.EOF {
		return nil, err
	}
	if n <= 0 {
		if n < 0 {
			panic("ReadAt returned negative n")
		}
		return nil, errEmptyBuf
	}
	offs := potentialOffsets(d, p[:n])
	m, err := bestMatch(d, offs)
	if err == errNoMatch {
		return lit{b: p[0]}, nil
	}
	if err != nil {
		return nil, err
	}
	return m, nil
}

func (g greedyFinder) findOps(s *State, all bool) (ops []operation, err error) {
	sd, ok := s.dict.(*hashDict)
	if !ok {
		panic("state doesn't contain hashDict")
	}
	d := *sd
	for d.head < d.buf.top {
		op, err := findOp(&d)
		if err != nil {
			return nil, err
		}
		if _, err = d.move(op.Len()); err != nil {
			return nil, err
		}
		ops = append(ops, op)
	}
	if !all && len(ops) > 0 {
		ops = ops[:len(ops)-1]
	}
	return ops, nil
}

func (g greedyFinder) String() string { return "greedy finder" }
