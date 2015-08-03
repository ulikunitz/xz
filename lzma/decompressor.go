package lzma

import "fmt"

// DecompressorParams provides the parameters for the decompressor
// instances.
type DecompressorParams struct {
	LC       int
	LP       int
	PB       int
	DictSize int64
	// length of compressed stream in bytes, 0 means undefined
	CompressedLen int64
	// length of uncompressed stream in bytes, 0 means undefined
	UncompressedLen int64
	// EOS expected; must be set if compressed lenght is undefined
	EOS bool
}

// Properties returns the LZMA properties as a single byte.
func (p *DecompressorParams) Properties() Properties {
	props, err := NewProperties(p.LC, p.LP, p.PB)
	if err != nil {
		panic(err)
	}
	return props
}

// SetProperties sets the LC, LB and PB fields from a properties byte.
func (p *DecompressorParams) SetProperties(props Properties) {
	p.LC, p.LP, p.PB = props.LC(), props.LP(), props.PB()
}

// Verify checks parameters for errors.
func (p *DecompressorParams) Verify() error {
	if p == nil {
		return lzmaError{"parameters must be non-nil"}
	}
	if err := verifyProperties(p.LC, p.LP, p.PB); err != nil {
		return err
	}
	if !(MinDictSize <= p.DictSize && p.DictSize <= MaxDictSize) {
		return rangeError{"DictSize", p.DictSize}
	}
	if p.DictSize != int64(int(p.DictSize)) {
		return lzmaError{fmt.Sprintf("DictSize %d too large for int",
			p.DictSize)}
	}
	if p.CompressedLen < 0 {
		return negError{"CompressedLen", p.CompressedLen}
	}
	if p.UncompressedLen < 0 {
		return negError{"UncompressedLen", p.UncompressedLen}
	}
	return nil
}

// Decompressor represents the Decompressor status.
type Decompressor struct {
}
