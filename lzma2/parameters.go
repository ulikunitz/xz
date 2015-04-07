package lzma2

import "github.com/uli-go/xz/lzbase"

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
	// buffer size
	BufferSize int64
}

// Properties returns LC, LP and PB as a Properties value.
func (p *Parameters) Properties() lzbase.Properties {
	props, err := lzbase.NewProperties(p.LC, p.LP, p.PB)
	if err != nil {
		panic(err)
	}
	return props
}

// SetProperties sets the LC, LP and PB fields.
func (p *Parameters) SetProperties(props lzbase.Properties) {
	p.LC, p.LP, p.PB = props.LC(), props.LP(), props.PB()
}

// normalizeSize puts the size on a normalized size. If DictSize and BufferSize
// are zero, then it is set to the value in Default. If both size values are
// too small they will set to the minimum size possible. Note that a buffer
// size less then zero will be ignored and will cause an error by
// verifyParameters.
func normalizeSizes(p *Parameters) {
	if p.DictSize == 0 {
		p.DictSize = Default.DictSize
	}
	if p.DictSize < lzbase.MinDictSize {
		p.DictSize = lzbase.MinDictSize
	}
	if p.BufferSize == 0 {
		p.BufferSize = Default.BufferSize
	}
	if 0 <= p.BufferSize && p.BufferSize < lzbase.MinLength {
		p.BufferSize = lzbase.MaxLength
	}
}

// verifyParameters checks parameters for errors.
func verifyParameters(p *Parameters) error {
	if p == nil {
		return newError("parameters must be non-nil")
	}
	if err := lzbase.VerifyProperties(p.LC, p.LP, p.PB); err != nil {
		return err
	}
	if !(lzbase.MinDictSize <= p.DictSize &&
		p.DictSize <= lzbase.MaxDictSize) {
		return newError("DictSize out of range")
	}
	hlen := int(p.DictSize)
	if hlen < 0 {
		return newError("DictSize cannot be converted into int")
	}
	if p.BufferSize <= 0 {
		return newError("buffer size must be positive")
	}
	return nil
}

// Default defines the parameters used by NewWriter.
var Default = Parameters{
	LC:         3,
	LP:         0,
	PB:         2,
	DictSize:   lzbase.MinDictSize,
	BufferSize: 4096,
}
