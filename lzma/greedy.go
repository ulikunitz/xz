package lzma

import (
	"errors"
	"io"
)

// greedyFinder is an OpFinder that implements a simple greedy algorithm
// to finding operations.
type greedyFinder struct{}

// Greedy provides a greedy operation finder.
var Greedy OpFinder

// don't want to expose the initialization of Greedy
func init() {
	Greedy = greedyFinder{}
}

type miniState struct {
	d hashDict
	r reps
}

func (ms *miniState) applyOp(op operation) {
	if _, err := ms.d.move(op.Len()); err != nil {
		panic(err)
	}
	ms.r.addOp(op)
}

// errNoMatch indicates that no match could be found
var errNoMatch = errors.New("no match found")

func weight(n, bits int) int {
	return (n << 20) / bits
}

// bestMatch provides the longest match reachable over the list of
// provided offsets.
func bestOp(ms *miniState, litop lit, offsets []int64) operation {
	op := operation(litop)
	w := weight(1, ms.r.opBits(op))
	prev := ms.d.head - 1
	for _, off := range offsets {
		n := ms.d.buf.equalBytes(ms.d.head, off, MaxLength)
		if n == 0 {
			continue
		}
		dist := uint32(prev - off)
		if n == 1 && dist != ms.r[0] {
			continue
		}
		m := match{distance: int64(dist) + 1, n: n}
		v := weight(m.n, ms.r.opBits(m))
		if v > w {
			w = v
			op = m
		}
	}
	return op
}

// errEmptyBuf indicates an empty buffer.
var errEmptyBuf = errors.New("empty buffer")

// potentialOffsets returns a list of offset positions where a match to
// at the current dictionary head can be identified.
func potentialOffsets(ms *miniState, p []byte) []int64 {
	prev, start := ms.d.head-1, ms.d.start()
	offs := make([]int64, 0, 32)
	var off int64
	// add repetitions
	for _, dist := range ms.r {
		off = prev - int64(dist)
		if off >= start {
			offs = append(offs, off)
		}
	}
	off = prev - 9
	if off < start {
		off = start
	}
	for ; off <= prev; off++ {
		offs = append(offs, off)
	}
	if len(p) == 4 {
		// distances from the hash table
		offs = append(offs, ms.d.t4.Offsets(p)...)
	}
	return offs
}

// finds a single operation at the current head of the hash dictionary.
func findOp(ms *miniState) operation {
	p := make([]byte, 4)
	n, err := ms.d.buf.ReadAt(p, ms.d.head)
	if err != nil && err != io.EOF {
		panic(err)
	}
	if n <= 0 {
		if n < 0 {
			panic("ReadAt returned negative n")
		}
		panic(errEmptyBuf)
	}
	offs := potentialOffsets(ms, p[:n])
	op := bestOp(ms, lit{p[0]}, offs)
	return op
}

// findOps identifies a sequence of operations starting at the current
// head of the dictionary stored in s. If all is set the whole data
// buffer will be covered, if it is not set the last operation reaching
// the head will not be output. This functionality has been included to
// support the extension of the last operation if new data comes in.
func (g greedyFinder) findOps(s *State, all bool) []operation {
	sd, ok := s.dict.(*hashDict)
	if !ok {
		panic("state doesn't contain hashDict")
	}
	ms := miniState{d: *sd, r: reps(s.rep)}
	ops := make([]operation, 0, 256)
	for ms.d.head < ms.d.buf.top {
		op := findOp(&ms)
		ms.applyOp(op)
		ops = append(ops, op)
	}
	if !all && len(ops) > 0 {
		ops = ops[:len(ops)-1]
	}
	return ops
}

// String implements the string function for the greedyFinder.
func (g greedyFinder) String() string { return "greedy finder" }
