package lzb

import (
	"errors"
)

// Parameters contain all information required to decode or encode an LZMA
// stream.
//
// The DictSize will be limited by MaxInt32 on 32-bit platforms.
type Parameters struct {
	// number of literal context bits
	LC int
	// number of literal position bits
	LP int
	// number of position bits
	PB int
	// size of the dictionary in bytes
	DictSize int64
	// size of uncompressed data in bytes
	Size int64
	// header includes unpacked size
	SizeInHeader bool
	// end-of-stream marker requested
	EOS bool
	// buffer size
	BufferSize int64
}

// Properties returns LC, LP and PB as Properties value.
func (p *Parameters) Properties() Properties {
	props, err := NewProperties(p.LC, p.LP, p.PB)
	if err != nil {
		panic(err)
	}
	return props
}

// SetProperties sets the LC, LP and PB fields.
func (p *Parameters) SetProperties(props Properties) {
	p.LC, p.LP, p.PB = props.LC(), props.LP(), props.PB()
}

// NormalizeSize sets the sizes to normal values. If DictSize or BufferSize
// are zero, then they values in Default are used. If both size values are
// too small they will set to the minimum size possible. BufferSize will
// at least have the same size as the DictSize.
func (p *Parameters) NormalizeSizes() {
	if p.BufferSize == 0 {
		p.BufferSize = Default.BufferSize
	}
	if p.BufferSize < MaxLength {
		p.BufferSize = MaxLength
	}
	if p.DictSize == 0 {
		p.DictSize = Default.DictSize
	}
	if p.DictSize < MinDictSize {
		p.DictSize = MinDictSize
	}
	if p.BufferSize < p.DictSize {
		p.BufferSize = p.DictSize
	}
}

// Verify checks parameters for errors.
func (p *Parameters) Verify() error {
	if p == nil {
		return errors.New("parameters must be non-nil")
	}
	if err := VerifyProperties(p.LC, p.LP, p.PB); err != nil {
		return err
	}
	if !(MinDictSize <= p.DictSize &&
		p.DictSize <= MaxDictSize) {
		return errors.New("DictSize out of range")
	}
	hlen := int(p.DictSize)
	if hlen < 0 {
		return errors.New("DictSize cannot be converted into int")
	}
	if p.Size < 0 {
		return errors.New("Size must not be negative")
	}
	if p.BufferSize < p.DictSize {
		return errors.New(
			"BufferSize must be equal or greater than DictSize")
	}
	return nil
}

// Default defines standard parameters.
var Default = Parameters{
	LC:         3,
	LP:         0,
	PB:         2,
	DictSize:   MinDictSize,
	BufferSize: 4096,
}
