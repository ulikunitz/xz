package lzma

import (
	"errors"
	"fmt"
)

// Properties define the properties for the LZMA and LZMA2 compression.
type Properties struct {
	LC int
	LP int
	PB int
}

// Returns the byte that encodes the properties.
func (p Properties) byte() byte {
	return (byte)((p.PB*5+p.LP)*9 + p.LC)
}

// fromByte reads the properties from a byte.
func (p *Properties) fromByte(b byte) error {
	p.LC = int(b % 9)
	b /= 9
	p.LP = int(b % 5)
	b /= 5
	p.PB = int(b)
	if p.PB > 4 {
		return errors.New("lzma: invalid properties byte")
	}
	return nil
}

// Verify verifies the correctnewss of the properties. It doesn't check the LZMA2
// condition that LC + LP <= 4.
func (p Properties) Verify() error {
	if !(0 <= p.LC && p.LC <= 8) {
		return fmt.Errorf("lzma: LC out of range 0..8")
	}
	if !(0 <= p.LP && p.LP <= 4) {
		return fmt.Errorf("lzma: LP out of range 0..4")
	}
	if !(0 <= p.PB && p.PB <= 4) {
		return fmt.Errorf("lzma: PB out of range 0..4")
	}
	return nil
}