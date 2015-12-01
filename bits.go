package xz

import "errors"

// errEncodeU64ShortBuffer indicates a short buffer condition
var errEncodeU64ShortBuffer = errors.New("short buffer")

// encodeU64 encodes a variable length encoded uint64
func encodeU64(p []byte, u uint64) (n int, err error) {
	i := 0
	for u >= 0x80 {
		if i > len(p) {
			return 0, errEncodeU64ShortBuffer
		}
		p[i] = byte(u) | 0x80
		i++
		u >>= 7
	}
	if i > len(p) {
		return 0, errEncodeU64ShortBuffer
	}
	p[i] = byte(u)
	i++
	return i, nil
}

// errors for decodeU64 function
var (
	errDecodeU64ShortBuffer = errors.New("decodeU64: short buffer")
	errDecodeU64NullByte    = errors.New("decodeU64: unexpected null byte")
)

// decodeU64 decodes a variable length encoded uint64
func decodeU64(p []byte) (u uint64, n int, err error) {
	if len(p) == 0 {
		return 0, 0, errDecodeU64ShortBuffer
	}
	i := 0
	u = uint64(p[i]) & 0x7f

	for p[i]&0x80 != 0 {
		i++
		if i > len(p) {
			return 0, 0, errDecodeU64ShortBuffer
		}
		if p[i] == 0 {
			return 0, 0, errDecodeU64NullByte
		}
		u |= (uint64(p[i]) & 0x7f) << (7 * uint(i))
	}
	i++
	return u, i, nil
}
