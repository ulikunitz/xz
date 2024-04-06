package lzma

import (
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
		return err
	}

	buf := make([]byte, 1<<14)
	for n > 0 {
		var p []byte
		if n < int64(len(buf)) {
			p = buf[:n]
		} else {
			p = buf
		}
		k, err := r.Read(p)
		if err != nil {
			return err
		}
		n -= int64(k)
	}

	return nil
}

// Stat2 returns information over the LZMA2 stream and consumes it in parallel.
func Stat2(r io.Reader) (info Info, err error) {
	for {
		h, err := parseChunkHeader(r)
		if err != nil {
			return Info{-1, -1}, err
		}
		switch h.control {
		case cEOS:
			info.Compressed += 1
			return info, nil
		case cU, cUD:
			info.Compressed += 3 + int64(h.size)
			err = skip(r, int64(h.size))
		case cC, cCS:
			info.Compressed += 5 + int64(h.compressedSize)
			err = skip(r, int64(h.compressedSize))
		case cCSP, cCSPD:
			info.Compressed += 6 + int64(h.compressedSize)
			err = skip(r, int64(h.compressedSize))
		default:
			panic("unexpected control byte")
		}
		if err != nil {
			return Info{-1, -1}, err
		}
		info.Uncompressed += int64(h.size)
	}
}
