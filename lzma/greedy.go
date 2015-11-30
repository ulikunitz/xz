// Copyright 2015 Ulrich Kunitz. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package lzma

// weight provides a function to compute the weight of an operation with
// length n that can be encoded with the given number of bits.
func weight(n, bits int) int {
	return (n << 20) / bits
}

// bestMatch provides the longest match reachable over the list of
// provided dists
func bestOp(d *EncoderDict, distances []int) operation {
	op := operation(lit{d.literal()})
	w := weight(1, d.reps.opBits(op))
	for _, distance := range distances {
		n := d.matchLen(distance)
		switch n {
		case 0:
			continue
		case 1:
			if uint32(distance-minDistance) != d.reps[0] {
				continue
			}
		}
		m := match{distance: distance, n: n}
		v := weight(n, d.reps.opBits(m))
		if v > w {
			w = v
			op = m
		}
	}
	return op
}

// findOp finds a single operation at the current head of the hash dictionary.
func findOp(d *EncoderDict, distances []int) operation {
	n := d.matches(distances)
	distances = distances[:n]
	// add small distances
	distances = append(distances, 1, 2, 3, 4, 5, 6, 7, 8)
	op := bestOp(d, distances)
	return op
}

func addOp(d *EncoderDict, op operation) {
	if err := d.writeOp(op); err != nil {
		panic(err)
	}
}

// greedy creates operations until the buffer is full. The function
// returns true if the end of the buffer has been reached.
func greedy(d *EncoderDict, f compressFlags) (end bool) {
	if d.bufferedAtFront() == 0 {
		return true
	}
	distances := make([]int, maxMatches, maxMatches+10)
	for d.ops.available() > 0 {
		op := findOp(d, distances)
		m := d.bufferedAtFront()
		if op.Len() >= m {
			if op.Len() > m {
				panic("op length exceed buffered")
			}
			if f&all != 0 {
				addOp(d, op)
			}
			return true
		}
		addOp(d, op)
	}
	return false
}
