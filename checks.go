package xz

import (
	"hash"
	"hash/crc32"
	"hash/crc64"
	"io"
)

// checksumCRC32 computes the CRC32 checksum as required for the xz format.
func checksumCRC32(data []byte) uint32 {
	return crc32.ChecksumIEEE(data)
}

// le32 converts the data slice in an unsigned 32-bit-integer. The integer must
// be stored in little-endian mode in the data slice. The function panics if
// data has not the length 4.
func le32(data []byte) uint32 {
	if len(data) != 4 {
		panic("data has not the length 4")
	}
	return uint32(data[0]) | uint32(data[1])<<8 | uint32(data[2])<<16 |
		uint32(data[3])<<24
}

// hashReader provides a reader that computes a hash (checksum) in parallel.
type hashReader struct {
	r    io.Reader
	hash hash.Hash
}

// Read implements the io.Reader interface. It delegates the Read to the reader
// provided, but updates the hash as well.
func (h *hashReader) Read(p []byte) (n int, err error) {
	n, err = h.r.Read(p)
	if n > 0 {
		h.hash.Write(p[:n])
	}
	return
}

// Sum provides the same functionality as the Sum function in the hash.Hash
// interface. Note that in the xz file format CRC values are stored in little
// endian order, but the packages crc32 and crc64 store them in big endian
// order. The leHash type exists for that reason.
func (h *hashReader) Sum(b []byte) []byte {
	return h.hash.Sum(b)
}

// Size returns the size of the hash.
func (h *hashReader) Size() int {
	return h.hash.Size()
}

// newHashReader creates a new hash reader.
func newHashReader(r io.Reader, hash hash.Hash) *hashReader {
	return &hashReader{r, hash}
}

// leHash wraps a Hash but reverts the Sum output. This allows the conversion
// of a Hash with big endian output to a hash with little endian output.
type leHash struct {
	hash.Hash
}

// reverseBytes reverts the bytes in the slice b.
func reverseBytes(b []byte) {
	for i, j := 0, len(b)-1; i < j; i, j = i+1, j-1 {
		b[i], b[j] = b[j], b[i]
	}
}

// Sum adds the actual hash (aka as checksum) to the slice b while reverting
// it.
func (h *leHash) Sum(b []byte) []byte {
	c := h.Hash.Sum(b)
	reverseBytes(c[len(b):])
	return c
}

// newCRC32Reader creates a new hash reader outputting the CRC32 checks. The
// IEEE polynomial is used.
func newCRC32Reader(r io.Reader) *hashReader {
	return &hashReader{r, &leHash{crc32.NewIEEE()}}
}

// ecmaTab stores the tab for the ECMA polynomical.
var ecmaTab = crc64.MakeTable(crc64.ECMA)

// newCRC64Reader creates a hashReader compute CRC64 checksums. The function
// uses the ECMA polynomial.
func newCRC64Reader(r io.Reader) *hashReader {
	return &hashReader{r, &leHash{crc64.New(ecmaTab)}}
}
