package lzma

import (
	"fmt"
	"io"

	"github.com/ulikunitz/xz/internal/stream"
)

// Info stores the uncompressed size and the compressed size of a LZMA2 stream.
type Info struct {
	Uncompressed int64
	Compressed   int64
}

// Walk2 visits all chunk headers of a LZMA2 stream.
func Walk2(r io.Reader, ch func(ChunkHeader) error) (n int64, err error) {
	s := stream.Wrap(r)
	n = s.Offset()
	for {
		h, err := parseChunkHeader(s)
		if err != nil {
			return s.Offset() - n, err
		}
		if err = ch(h); err != nil {
			return s.Offset() - n, err
		}
		switch h.Control {
		case CEOS:
			return s.Offset() - n, nil
		case CU, CUD:
			_, err = s.Discard64(int64(h.Size))
		case CC, CCS, CCSP, CCSPD:
			_, err = s.Discard64(int64(h.CompressedSize))
		default:
			panic("unexpected control byte")
		}
		if err != nil {
			return s.Offset() - n, err
		}
	}
}

// Stat2 returns information over the LZMA2 stream and consumes it in parallel.
func Stat2(r io.Reader) (info Info, err error) {
	_, err = Walk2(r, func(h ChunkHeader) error {
		info.Uncompressed += int64(h.Size)
		switch h.Control {
		case CU, CUD:
			info.Compressed += 3 + int64(h.Size)
		case CC, CCS:
			info.Compressed += 5 + int64(h.CompressedSize)
		case CCSP, CCSPD:
			info.Compressed += 6 + int64(h.CompressedSize)
		default:
			return fmt.Errorf("lzma: unexpected control byte %#02x", h.Control)
		}
		return nil
	})
	return info, err
}
