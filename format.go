package xz

import (
	"github.com/ulikunitz/xz/xzinternals"
)

// HeaderLen provides the length of the xz file header.
const HeaderLen = xzinternals.HeaderLen

// Constants for the checksum methods supported by xz.
const (
	None   = xzinternals.None
	CRC32  = xzinternals.CRC32
	CRC64  = xzinternals.CRC64
	SHA256 = xzinternals.SHA256
)

// ValidHeader checks whether data is a correct xz file header. The
// length of data must be HeaderLen.
func ValidHeader(data []byte) bool {
	var h xzinternals.Header
	err := h.UnmarshalBinary(data)
	return err == nil
}
