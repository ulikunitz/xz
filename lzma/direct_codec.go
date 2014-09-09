package lzma

import "fmt"

// directEncoder supports the "direct" encoding of values with a fixed number of
// bits. Direct encoding means the use of the range encoder with probability
// 1/2 for zero and one bits. The supported range of byte values is [1,32].
type directEncoder byte

// makeDirectEncoder returns a directEncoder. The function panics if the bits
// argument is outside of the range [1,32].
func makeDirectEncoder(bits int) directEncoder {
	if !(1 <= bits && bits <= 32) {
		panic(fmt.Errorf("bits=%d out of range", bits))
	}
	return directEncoder(bits)
}

// Returns the number of bits for which encoding is supported.
func (de directEncoder) Bits() int {
	return int(de)
}

// Encode uses the range encoder to encode a value with the fixed number of
// bits. The most-significant bit is encoded first.
func (de directEncoder) Encode(v uint32, e *rangeEncoder) error {
	for i := int(de) - 1; i >= 0; i-- {
		if err := e.DirectEncodeBit(v >> uint(i)); err != nil {
			return err
		}
	}
	return nil
}

// directDecoder support the "direct" decoding of values with a fixed number of
// bits.
type directDecoder byte

// makeDirectDecoder returns a directDecoder. The function panics if the bits
// argument is outside the range [1,32.].
func makeDirectDecoder(bits int) directDecoder {
	if !(1 <= bits && bits <= 32) {
		panic(fmt.Errorf("bits=%d out of range", bits))
	}
	return directDecoder(bits)
}

// Bits returns the number of bits for which values can be decoded.
func (dd directDecoder) Bits() int {
	return int(dd)
}

// Decode uses the range decoder to decode a value with the given number of
// given bits. The most-significant bit is decoded first.
func (dd directDecoder) Decode(d *rangeDecoder) (v uint32, err error) {
	for i := int(dd) - 1; i >= 0; i-- {
		x, err := d.DirectDecodeBit()
		if err != nil {
			return 0, err
		}
		v = (v << 1) | x
	}
	return v, nil
}
