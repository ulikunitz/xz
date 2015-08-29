// Copyright 2015 Ulrich Kunitz. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package lzma

// greedyFinder is an OpFinder that implements a simple greedy algorithm
// to finding operations.
type greedyFinder struct{}

// check that greadyFinder satisfies the opFinder interface.
// var _ opFinder = greedyFinder{}

// miniState represents a minimal state to be used by optimizer.
type miniState struct {
	d encoderDict
	r reps
}

// applyOp applies the LZMA operation to the miniState.
func (ms *miniState) applyOp(op operation) {
	ms.d.Advance(op.Len())
	ms.r.addOp(op)
}

// weight provides a function to compute the weight of an operation with
// length n that can be encoded with the given number of bits.
func weight(n, bits int) int {
	return (n << 20) / bits
}

// bestMatch provides the longest match reachable over the list of
// provided dists
func bestOp(ms *miniState, literal byte, distances []int) operation {
	op := operation(lit{literal})
	w := weight(1, ms.r.opBits(op))
	for _, distance := range distances {
		n := ms.d.MatchLen(distance)
		if n == 0 {
			continue
		}
		if n == 1 && uint32(distance-minDistance) != ms.r[0] {
			continue
		}
		m := match{distance: distance, n: n}
		v := weight(n, ms.r.opBits(m))
		if v > w {
			w = v
			op = m
		}
	}
	return op
}

// findOp finds a single operation at the current head of the hash dictionary.
func findOp(ms *miniState) operation {
	l := ms.d.Literal()
	distances := ms.d.Matches()
	// add small distances
	distances = append(distances, 1, 2, 3, 4, 5, 6, 7, 8)
	op := bestOp(ms, l, distances)
	return op
}

// findOps identifies a sequence of operations starting at the current
// head of the dictionary stored in s. If all is set the whole data
// buffer will be covered, if it is not set the last operation reaching
// the head will not be output. This functionality has been included to
// support the extension of the last operation if new data comes in.
func (g greedyFinder) findOps(d *encoderDict, r reps, all bool) []operation {
	ms := miniState{d: *d, r: r}
	ops := make([]operation, 0, 256)
	if ms.d.Buffered() > 0 {
		for {
			op := findOp(&ms)
			if op.Len() >= ms.d.Buffered() {
				if op.Len() > ms.d.Buffered() {
					panic("op length exceeds buffered")
				}
				if all {
					ms.applyOp(op)
					ops = append(ops, op)
				}
				break

			}
			ms.applyOp(op)
			ops = append(ops, op)
		}
	}
	return ops
}

// name returns "greedy" for the greedy finder.
func (g greedyFinder) name() string { return "greedy" }
