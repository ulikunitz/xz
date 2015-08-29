package lzma

import (
	"errors"
	"io"
)

// MinDictCap and MaxDictCap provide the range of supported dictionary
// capacities.
const (
	MinDictCap = 1 << 12
	MaxDictCap = 1<<32 - 1
)

// Parameters control the encoding of a LZMA stream.
type Parameters struct {
	LC                     int
	LP                     int
	PB                     int
	DictCap                int
	UncompressedSize       int64
	EOS                    bool
	IgnoreUncompressedSize bool
}

// verifyParameters verify the parameters provided by the end user.
func verifyParameters(p *Parameters) error {
	if err := verifyProperties(p.LC, p.LP, p.PB); err != nil {
		return err
	}
	if !(MinDictCap <= p.DictCap && p.DictCap <= MaxDictCap) {
		return errors.New("DictCap out of range")
	}
	if p.IgnoreUncompressedSize {
		if !p.EOS {
			return errors.New("if no uncompressed size is given, " +
				"EOS must be set")
		}
	} else {
		if p.UncompressedSize < 0 {
			return errors.New(
				"UncompressedSize must be greate or equal 0")
		}
	}
	return nil
}

// Default defines the default parameters for the LZMA writer.
var Default = Parameters{
	LC:      3,
	LP:      0,
	PB:      2,
	DictCap: 8 * 1024 * 1024,
	EOS:     true,
	IgnoreUncompressedSize: true,
}

// convertParameters converts the parameters into the parameters for the
// Encoder.
func convertParameters(p *Parameters) CodecParams {
	c := CodecParams{
		DictCap:          p.DictCap,
		BufCap:           p.DictCap + 1<<13,
		UncompressedSize: p.UncompressedSize,
		LC:               p.LC,
		LP:               p.LP,
		PB:               p.PB,
		Flags:            CNoCompressedSize,
	}
	if p.EOS {
		c.Flags |= CEOSMarker
	}
	if p.IgnoreUncompressedSize {
		c.Flags |= CNoUncompressedSize
	}
	return c
}

// Writer compresses data in the classic LZMA format.
type Writer struct {
	e Encoder
}

// NewWriter creates a new writer for the classic LZMA format.
func NewWriter(lzma io.Writer) (w *Writer, err error) {
	return NewWriterParams(lzma, &Default)
}

// NewWriterParams supports parameters for the creation of an LZMA
// writer.
func NewWriterParams(lzma io.Writer, p *Parameters) (w *Writer, err error) {
	if err = verifyParameters(p); err != nil {
		return nil, err
	}
	cparams := convertParameters(p)
	if err := writeHeader(lzma, &cparams); err != nil {
		return nil, err
	}
	w = new(Writer)
	if err = InitEncoder(&w.e, lzma, &cparams); err != nil {
		return nil, err
	}
	return w, nil
}

// Write puts data into the Writer.
func (w *Writer) Write(p []byte) (n int, err error) {
	return w.e.Write(p)
}

// Close closes the writer stream. It ensures that all data from the
// buffer will be compressed and the LZMA stream will be finished.
func (w *Writer) Close() error {
	if w.e.flags&CNoUncompressedSize == 0 {
		if w.e.Uncompressed()+int64(w.e.Buffered()) != w.e.uncompressedSize {
			return ErrUncompressedSize
		}
	}
	return w.e.Close()
}
