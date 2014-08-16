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
)

// streamFlags represents the flags for the stream.
type streamFlags uint16

// check returns the check value for the stream flags.
func (sf streamFlags) check() byte {
	return byte(sf & 0x0f)
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
	_, err = io.ReadFull(r, buf)
	if err != nil {
		return 0, fmt.Errorf("xz stream header: %s", err)
	}
	if !bytes.Equal(buf[:magicLen], magic) {
		return 0, errors.New("xz stream header: magic mismatch")
	}
	cs := checksumCRC32(buf[magicLen : magicLen+flagLen])
	csWant := le32(buf[headerLen-4:])
	if cs != csWant {
		return 0, fmt.Errorf("xz stream header: CRC32 error")
	}
	if buf[magicLen] != 0 {
		return 0, errors.New(
			"xz stream header: non-zero reserved flag bit")
	}
	sf = streamFlags(buf[magicLen+1])
	if sf&^streamFlags(0x0f) != 0 {
		return 0, errors.New(
			"xz stream header: non-zero reserved flag bit")
	}
	switch sf.check() {
	case chkNone, chkCRC32, chkCRC64, chkSHA256:
	default:
		return 0, errors.New("xz stream header: invalid check value")
	}
	return sf, nil
}
