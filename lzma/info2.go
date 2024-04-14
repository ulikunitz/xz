package lzma

import (
	"fmt"
	"io"
)

// Info stores the uncompressed size and the compressed size of a LZMA2 stream.
type Info struct {
	Uncompressed int64
	Compressed   int64
}

// skip skips over the next n bytes in the reader.
func skip(r io.Reader, n int64) error {
	if n < 0 {
		panic("n < 0")
	}

	if s, ok := r.(io.Seeker); ok {
		_, err := s.Seek(n, io.SeekCurrent)
		if err == nil {
			return nil
		}
	}

	p := make([]byte, 16*1024)
	for n > 0 {
		if n < int64(len(p)) {
			p = p[:n]
		}
		k, err := r.Read(p)
		n -= int64(k)
		if err != nil {
			return err
		}
	}

	return nil
}

// Walk2 visits all chunk headers of a LZMA2 stream.
func Walk2(r io.Reader, ch func(ChunkHeader) error) error {
	for {
		h, err := parseChunkHeader(r)
		if err != nil {
			return err
		}
		if err = ch(h); err != nil {
			return err
		}
		switch h.Control {
		case CEOS:
			return nil
		case CU, CUD:
			err = skip(r, int64(h.Size))
		case CC, CCS, CCSP, CCSPD:
			err = skip(r, int64(h.CompressedSize))
		default:
			panic("unexpected control byte")
		}
		if err != nil {
			return err
		}
	}
}

// Stat2 returns information over the LZMA2 stream and consumes it in parallel.
func Stat2(r io.Reader) (info Info, err error) {
	return info, Walk2(r, func(h ChunkHeader) error {
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
}
