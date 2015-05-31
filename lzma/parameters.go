package lzma

import "github.com/uli-go/xz/lzb"

// Parameters contain all information required to decode or encode an LZMA
// stream.
//
// The maximum of the dictionary and buffer size is 2^31-1 on 32-bit
// platforms.
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
	// extra buffer size on top of dictionary size
	ExtraBufSize int64
}

// Default defines standard parameters.
var Default = Parameters{
	LC:       3,
	LP:       0,
	PB:       2,
	DictSize: 8 * 1024 * 1024,
}

// lzbParamertes converts Parameters in the identical lzb.Parameters.
func lzbParameters(p *Parameters) lzb.Parameters {
	return lzb.Parameters{
		LC:           p.LC,
		LP:           p.LP,
		PB:           p.PB,
		DictSize:     p.DictSize,
		Size:         p.Size,
		SizeInHeader: p.SizeInHeader,
		EOS:          p.EOS,
		ExtraBufSize: p.ExtraBufSize,
	}
}
