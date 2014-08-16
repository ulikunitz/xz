package xz

import "hash/crc32"

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
