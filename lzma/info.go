package lzma

import (
	"errors"
	"io"
	"math"

	"github.com/ulikunitz/xz/internal/stream"
)

// Stat provides the size of the uncompressed file as well as the compressed
// size of an lzma file. The function doesn't check for decompression errors.
func Stat(r io.Reader) (info Info, err error) {
	z := stream.Wrap(r)
	start := z.Offset()

	var p = make([]byte, headerLen)
	if _, err = io.ReadFull(z, p); err != nil {
		return info, err
	}
	var params params
	if err = params.UnmarshalBinary(p); err != nil {
		return info, err
	}
	if err = params.Verify(); err != nil {
		return info, err
	}

	switch {
	case params.uncompressedSize == EOSSize:
		info.Uncompressed = -1
	case params.uncompressedSize <= math.MaxInt64:
		info.Uncompressed = int64(params.uncompressedSize)
	default:
		return info, errors.New("lzma: size overflow")
	}

	if info.Uncompressed >= 0 {
		if s, ok := r.(io.Seeker); ok {
			end, err := s.Seek(0, io.SeekEnd)
			if err == nil {
				info.Compressed = end - start
				return info, nil
			}
		}
	}

	if uint64(params.dictSize) > math.MaxInt {
		return info, errors.New("lzma: dictSize too large")
	}
	d := int(params.dictSize)

	rr := new(reader)
	err = rr.init(z, d, params.props, params.uncompressedSize)
	if err != nil {
		return info, err
	}

	n, err := io.Copy(io.Discard, rr)
	info.Compressed = z.Offset() - start
	if info.Uncompressed < 0 {
		info.Uncompressed = n
	} else if n != info.Uncompressed {
		return info, errors.New("lzma: uncompressed size mismatch")
	}
	return info, err
}
