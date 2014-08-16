package xz

import (
	"bytes"
	"errors"
	"fmt"
	"io"
)

// Constants for the check values of the stream flags.
const (
	chkNone   = 0x00
	chkCRC32  = 0x01
	chkCRC64  = 0x04
	chkSHA256 = 0x0a
	chkMask   = 0x0f
)

// streamFlags represents the flags for the stream.
type streamFlags byte

// check returns the check value for the stream flags.
func (sf streamFlags) check() byte {
	return byte(sf) & chkMask
}

// String represents the stream flags as string.
func (sf streamFlags) String() string {
	var s string
	switch sf.check() {
	case chkNone:
		s = "None"
	case chkCRC32:
		s = "CRC32"
	case chkCRC64:
		s = "CRC64"
	case chkSHA256:
		s = "SHA-256"
	default:
		s = "Reserved"
	}
	return s
}

func readStreamFlags(data []byte) (sf streamFlags, err error) {
	if len(data) != 2 {
		return 0, errors.New("readStreamFlags: data must have length 2")
	}
	if data[0] != 0 {
		return 0, errors.New(
			"stream flags: first reserved byte non-zero")
	}
	sf = streamFlags(data[1])
	if sf&^chkMask != 0 {
		return 0, errors.New(
			"stream flags: reserved bits in second byte non-zero")
	}
	switch sf.check() {
	case chkNone, chkCRC32, chkCRC64, chkSHA256:
	default:
		return 0, errors.New("stream flags: invalid check value")
	}
	return sf, nil
}

// readStreamHeader reads the xz stream header and returns a representation of
// the stream flags. The function returns an error if the header cannot be read
// or the stream flags are invalid.
func readStreamHeader(r io.Reader) (sf streamFlags, err error) {
	magic := []byte{0xfd, '7', 'z', 'X', 'Z', 0x00}
	magicLen := len(magic)
	const (
		flagLen   = 2
		headerLen = 12
	)
	buf := make([]byte, headerLen)
	if _, err = io.ReadFull(r, buf); err != nil {
		return 0, fmt.Errorf("xz stream header: %s", err)
	}
	if !bytes.Equal(buf[:magicLen], magic) {
		return 0, errors.New("xz stream header: magic mismatch")
	}
	cs := checksumCRC32(buf[magicLen : magicLen+flagLen])
	csWant := le32(buf[headerLen-4:])
	if cs != csWant {
		return 0, errors.New("xz stream header: CRC32 error")
	}
	sf, err = readStreamFlags(buf[magicLen : magicLen+2])
	if err != nil {
		return 0, fmt.Errorf("xz stream header: %s", err)
	}
	return sf, nil
}

func readStreamFooter(r io.Reader) (
	backwardSize int64, sf streamFlags, err error,
) {
	magic := []byte{'Y', 'Z'}
	magicLen := len(magic)
	const (
		crc32Len = 4
		bsLen    = 4
		flagLen  = 2
	)
	footerLen := crc32Len + bsLen + flagLen + magicLen
	buf := make([]byte, footerLen)
	if _, err = io.ReadFull(r, buf); err != nil {
		return 0, 0, fmt.Errorf("xz stream footer: %s", err)
	}
	if !bytes.Equal(buf[footerLen-magicLen:], magic) {
		return 0, 0, errors.New("xz stream footer: magic mismatch")
	}
	cs := checksumCRC32(buf[crc32Len : footerLen-magicLen])
	csWant := le32(buf[:crc32Len])
	if cs != csWant {
		return 0, 0, errors.New("xz stream footer: CRC32 error")
	}
	backwardSize = int64(le32(buf[crc32Len : crc32Len+bsLen]))
	backwardSize = 4 * (backwardSize + 1)
	sf, err = readStreamFlags(buf[crc32Len+bsLen : crc32Len+bsLen+2])
	if err != nil {
		return 0, 0, fmt.Errorf("xz stream header: %s", err)
	}
	return backwardSize, sf, nil
}
