package lzma

import (
	"fmt"
	"io"
)

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
	state      *State
	rd         *rangeDecoder
	properties Properties
	dict       *syncDict
	// read head
	head int64
	// start is the offset from the start of the uncompressed stream
	start int64
	// length gives the length of the uncompressed stream; if 0 no
	// length has been given
	length int64
	// filled marks decompressors that will not get more data put
	// into the dictionary
	filled bool
	// eos is set if a eos marker is expected
	eos bool
}

func NewDecompressor(lzma io.Reader, p DecompressorParams) (d *Decompressor, err error) {
	if err = p.Verify(); err != nil {
		return nil, err
	}
	buf, err := newBuffer(p.DictSize)
	if err != nil {
		return
	}
	dict, err := newSyncDict(buf, p.DictSize)
	if err != nil {
		return
	}
	props := p.Properties()
	state := NewState(props, dict)
	// TODO: convert lzma into a limited reader if CompressedLen > 0
	lr := lzma
	if p.CompressedLen > 0 {
		lr = io.LimitReader(lzma, p.CompressedLen)
	}
	rd, err := newRangeDecoder(lr)
	if err != nil {
		return nil, err
	}
	d = &Decompressor{
		state:      state,
		rd:         rd,
		properties: props,
		dict:       dict,
		length:     p.UncompressedLen,
		eos:        p.EOS,
	}
	return d, nil
}

func (d *Decompressor) Read(p []byte) (n int, err error) {
	panic("TODO")
}

func (d *Decompressor) Decompress() error {
	panic("TODO")
}
