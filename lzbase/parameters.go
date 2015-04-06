package lzbase

// Parameters provides a size if sizeInHeader is true. The size refers here to
// the uncompressed size.
type Parameters struct {
	Size         int64
	SizeInHeader bool
	EOS          bool
}

// verifyParameters checks whether the Size field is not negative.
func verifyParameters(p *Parameters) error {
	if p.Size < 0 {
		return newError("parameter Size must not be negative")
	}
	return nil
}
