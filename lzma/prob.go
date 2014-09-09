package lzma

// movebits defines the number of bits used for the updates of probability
// values.
const movebits = 5

// probbits defines the number of bits of a probability value.
const probbits = 11

// probInit defines 0.5 as initial value for prob values.
const probInit prob = 1 << (probbits - 1)

// Type prob represents probabilities.
type prob uint16

// Dec decreases the probability. The decrease is proportional to the
// probability value.
func (p *prob) dec() {
	*p -= *p >> movebits
}

// Inc increases the probability. The Increase is proportional to the
// difference of 1 and the probability value.
func (p *prob) inc() {
	*p += ((1 << probbits) - *p) >> movebits
}

// Computes the new bound for a given range using the probability value.
func (p prob) bound(r uint32) uint32 {
	return (r >> probbits) * uint32(p)
}
