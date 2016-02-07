package lzma

import "errors"

// WriterParams describes the parameters for both LZMA writers.
type WriterParams struct {
	// The properties for the encoding. If the it is nil the value
	// {LC: 3, LP: 0, PB: 2} will be chosen.
	Properties *Properties
	// The capacity of the dictionary. If DictCap is zero, the value
	// 8 MiB will be chosen.
	DictCap int
	// Size of the lookahead buffer, the it is zero, the value will
	// be 4096.
	BufSize int
}

func fillWriterParams(p *WriterParams) *WriterParams {
	if p == nil {
		p = new(WriterParams)
	}
	if p.Properties == nil {
		p.Properties = &Properties{LC: 3, LP: 0, PB: 2}
	}
	if p.DictCap == 0 {
		p.DictCap = 8 * 1024 * 1024
	}
	if p.BufSize == 0 {
		p.BufSize = 4096
	}
	return p
}

// verifyDictCap verifies values for the dictionary capacity.
func verifyDictCap(dictCap int) error {
	if !(1 <= dictCap && int64(dictCap) <= MaxDictCap) {
		return errors.New("lzma: dictionary capacity is out of range")
	}
	return nil
}

func (p *WriterParams) verify() error {
	var err error

	if p == nil {
		return errors.New("lzma: parameters are nil")
	}

	if err = p.Properties.verify(); err != nil {
		return err
	}

	// dictionary capacity
	if err = verifyDictCap(p.DictCap); err != nil {
		return err
	}

	// buffer size
	if p.BufSize < 1 {
		return errors.New(
			"lzma: lookahead buffer size must be larger than zero")
	}

	return nil
}

func (p *WriterParams) verifyLZMA2() error {
	if err := p.verify(); err != nil {
		return err
	}
	if p.Properties.LC+p.Properties.LP > 4 {
		return errors.New("lzma: sum of lc and lp exceeds 4")
	}
	return nil
}

// ReaderParams defines the LZMA reader parameters.
type ReaderParams struct {
	DictCap int
}

func fillReaderParams(p *ReaderParams) *ReaderParams {
	if p == nil {
		p = new(ReaderParams)
	}
	if p.DictCap == 0 {
		p.DictCap = 8 * 1024 * 1024
	}
	return p
}

// verify verifies the LZMA2 reader parameters for correctness.
func (p *ReaderParams) verify() error {
	return verifyDictCap(p.DictCap)
}
