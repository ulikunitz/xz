package lzma

import (
	"io"
)

// Maximum and minimum values for individual parameters.
const (
	MinLC       = 0
	MaxLC       = 8
	MinLP       = 0
	MaxLP       = 4
	MinPB       = 0
	MaxPB       = 4
	MinDictSize = 1 << 12
	MaxDictSize = 1<<32 - 1
)

// Properties contains the parametes lc, lp and pb.
type Properties byte

// NewProperties returns a new properties value. It verifies the validity of
// the arguments.
func NewProperties(lc, lp, pb int) (p Properties, err error) {
	if err = verifyProperties(lc, lp, pb); err != nil {
		return
	}
	return Properties((pb*5+lp)*9 + lc), nil
}

// LC returns the number of literal context bits.
func (p Properties) LC() int {
	return int(p) % 9
}

// LP returns the number of literal position bits.
func (p Properties) LP() int {
	return (int(p) / 9) % 5
}

// PB returns the number of position bits.
func (p Properties) PB() int {
	return int(p) / 45
}

// verifyProperties checks the argument for any errors.
func verifyProperties(lc, lp, pb int) error {
	if !(MinLC <= lc && lc <= MaxLC) {
		return newError("lc out of range")
	}
	if !(MinLP <= lp && lp <= MaxLC) {
		return newError("lp out of range")
	}
	if !(MinPB <= pb && pb <= MaxPB) {
		return newError("pb out of range")
	}
	return nil
}

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
	DictSize uint32
	// size of uncompressed data in bytes
	Size int64
	// header includes unpacked size
	SizeInHeader bool
	// end-of-stream marker requested
	EOS bool
	// buffer size
	BufferSize int
}

// Properties returns lc, lp and pb as Properties value.
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

// normalizeSize puts the size on a normalized size. If DictSize and BufferSize
// are zero, then it is set to the value in Default. If both size values are
// too small they will set to the minimum size possible. Note that a buffer
// size less then zero will be ignored and will cause an error by
// verifyParameters.
func normalizeSizes(p *Parameters) {
	if p.DictSize == 0 {
		p.DictSize = Default.DictSize
	}
	if p.DictSize < MinDictSize {
		p.DictSize = MinDictSize
	}
	if p.BufferSize == 0 {
		p.BufferSize = Default.BufferSize
	}
	if 0 <= p.BufferSize && p.BufferSize < minLength {
		p.BufferSize = maxLength
	}
}

// verifyParameters checks parameters for errors.
func verifyParameters(p *Parameters) error {
	if p == nil {
		return newError("parameters must be non-nil")
	}
	if err := verifyProperties(p.LC, p.LP, p.PB); err != nil {
		return err
	}
	if !(MinDictSize <= p.DictSize && p.DictSize <= MaxDictSize) {
		return newError("DictSize out of range")
	}
	hlen := int(p.DictSize)
	if hlen < 0 {
		return newError("DictSize cannot be converted into int")
	}
	if p.Size < 0 {
		return newError("length must not be negative")
	}
	return nil
}

// Default defines the parameters used by NewWriter.
var Default = Parameters{
	LC:         3,
	LP:         0,
	PB:         2,
	DictSize:   MinDictSize,
	BufferSize: 4096,
}

// getUint32LE reads an uint32 integer from a byte slize
func getUint32LE(b []byte) uint32 {
	x := uint32(b[3]) << 24
	x |= uint32(b[2]) << 16
	x |= uint32(b[1]) << 8
	x |= uint32(b[0])
	return x
}

// getUint64LE converts the uint64 value stored as little endian to an uint64
// value.
func getUint64LE(b []byte) uint64 {
	x := uint64(b[7]) << 56
	x |= uint64(b[6]) << 48
	x |= uint64(b[5]) << 40
	x |= uint64(b[4]) << 32
	x |= uint64(b[3]) << 24
	x |= uint64(b[2]) << 16
	x |= uint64(b[1]) << 8
	x |= uint64(b[0])
	return x
}

// putUint32LE puts an uint32 integer into a byte slice that must have at least
// a lenght of 4 bytes.
func putUint32LE(b []byte, x uint32) {
	b[0] = byte(x)
	b[1] = byte(x >> 8)
	b[2] = byte(x >> 16)
	b[3] = byte(x >> 24)
}

// putUint64LE puts the uint64 value into the byte slice as little endian
// value. The byte slice b must have at least place for 8 bytes.
func putUint64LE(b []byte, x uint64) {
	b[0] = byte(x)
	b[1] = byte(x >> 8)
	b[2] = byte(x >> 16)
	b[3] = byte(x >> 24)
	b[4] = byte(x >> 32)
	b[5] = byte(x >> 40)
	b[6] = byte(x >> 48)
	b[7] = byte(x >> 56)
}

// readHeader reads the classic LZMA header.
func readHeader(r io.Reader) (p *Parameters, err error) {
	b := make([]byte, 13)
	_, err = io.ReadFull(r, b)
	if err != nil {
		return nil, err
	}
	p = new(Parameters)
	props := Properties(b[0])
	p.LC, p.LP, p.PB = props.LC(), props.LP(), props.PB()
	p.DictSize = getUint32LE(b[1:])
	u := getUint64LE(b[5:])
	if u == noHeaderLen {
		p.Size = 0
		p.EOS = true
		p.SizeInHeader = false
	} else {
		p.Size = int64(u)
		if p.Size < 0 {
			return nil, newError(
				"unpack length in header not supported by" +
					" int64")
		}
		p.EOS = false
		p.SizeInHeader = true
	}

	normalizeSizes(p)
	return p, nil
}

// writeHeader writes the header for classic LZMA files.
func writeHeader(w io.Writer, p *Parameters) error {
	var err error
	if err = verifyParameters(p); err != nil {
		return err
	}
	b := make([]byte, 13)
	b[0] = byte(p.Properties())
	putUint32LE(b[1:5], p.DictSize)
	var l uint64
	if p.SizeInHeader {
		l = uint64(p.Size)
	} else {
		l = noHeaderLen
	}
	putUint64LE(b[5:], l)
	_, err = w.Write(b)
	return err
}
