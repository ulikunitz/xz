package xz

import (
	"errors"
	"fmt"
	"io"
)

// blockFlags represents the block flags. Those flags define the number of
// filters used in the format.
type blockFlags byte

// reservedBits returns the reserved bits of the flags.
func (bf blockFlags) reservedBits() byte {
	return byte(bf) & 0x3c
}

// filters returns the number of filters in that block.
func (bf blockFlags) filters() int {
	return int(bf&0x03) + 1
}

// compressedSizePresent checks whether the compressed size field is present in
// the block header.
func (bf blockFlags) compressedSizePresent() bool {
	return bf&0x40 != 0
}

// uncompressedSizePresent checks whether the uncompressed size field is
// present in the block header.
func (bf blockFlags) uncompressedSizePresent() bool {
	return bf&0x80 != 0
}

// String provides a string representation for the blockFlags. The string
// "2/cu" describes the use of 2 filters and the presence of compressed and
// uncompressed size fields. The string "1/--" would describe a block flag with
// 1 filter and no size fields present.
func (bf blockFlags) String() string {
	c, u := '-', '-'
	if bf.compressedSizePresent() {
		c = 'c'
	}
	if bf.uncompressedSizePresent() {
		u = 'u'
	}
	return fmt.Sprintf("%d/%c%c", bf.filters(), c, u)
}

// A filterID can be quite long and is not restricted to the filter ids defined
// in the xz file format document.
type filterID uint64

// List of filter ids. Note that we didn't include all filters currently
// supported to reduce complexity.
const (
	idLZMA2 filterID = 0x21
)

// filterNames stores the names for the supported filter domains.
var filterNames = map[filterID]string{
	idLZMA2: "LZMA2 filter",
}

// String() provides a string presentation for the filter id.
func (id filterID) String() string {
	s, ok := filterNames[id]
	if !ok {
		return fmt.Sprintf("unknown filter (0x%x)", uint64(id))
	}
	return s
}

// filterFlags stores the properties of a single filter. Different filter
// types will have different properties. For that reason the filter flags are
// provided as interface and must be converted using type assertions to the
// specific type for the actual filter. But each filter type has its id.
type filterFlags interface {
	id() filterID
}

// readFilterFlags reads the flags for a single filter.
func readFilterFlags(r io.Reader, n int) (flags filterFlags, err error) {
	panic("TODO")
}

// blockInfo provides all information available in a block header.
type blockInfo struct {
	flags            blockFlags
	compressedSize   int64
	uncompressedSize int64
	filters          []filterFlags
}

// decodeInt64 decodes an encoded integer in the xz format.
func decodeInt64(r io.Reader) (n int64, err error) {
	var buf [1]byte
	for i := uint32(0); i < 9; i++ {
		_, err := io.ReadFull(r, buf[:1])
		if err != nil {
			return 0, err
		}
		b := buf[0]
		n |= int64(b&0x7f) << (7 * i)
		if b&0x80 == 0 {
			return n, nil
		}
	}
	return 0, errors.New("too many bytes in encoded integer")
}

// readBlockHeaderSize reads the block header size from the reader provided. It
// returns the size or an error if it occurs.
func readBlockHeaderSize(r io.Reader) (n int, err error) {
	var buf [1]byte
	_, err = io.ReadFull(r, buf[:1])
	if err != nil {
		return 0, err
	}
	n = 4 * (int(buf[0]) + 1)
	return n, nil
}

// readBlockFlags reads the block flags from the reader.
func readBlockFlags(r io.Reader) (bf blockFlags, err error) {
	var buf [1]byte
	_, err = io.ReadFull(r, buf[:1])
	if err != nil {
		return 0, err
	}
	bf = blockFlags(buf[0])
	if bf.reservedBits() != 0 {
		return 0, errors.New("block flags: reserved bits set")
	}
	return bf, nil
}

// readBlockHeader reads the block header. It returns a blockInfo value with
// all information provided by the block header.
func readBlockHeader(r io.Reader) (info *blockInfo, err error) {
	hr := newCRC32Reader(r)
	size, err := readBlockHeaderSize(hr)
	if err != nil {
		return nil, fmt.Errorf("xz block header: %s", err)
	}
	info = new(blockInfo)
	lr := &io.LimitedReader{R: hr, N: int64(size - 1)}
	info.flags, err = readBlockFlags(lr)
	if err != nil {
		return nil, fmt.Errorf("xz block header: %s", err)
	}
	if info.flags.compressedSizePresent() {
		if info.compressedSize, err = decodeInt64(lr); err != nil {
			return nil, fmt.Errorf(
				"xz block header: compressed size: %s", err)
		}
	}
	if info.flags.uncompressedSizePresent() {
		if info.uncompressedSize, err = decodeInt64(lr); err != nil {
			return nil, fmt.Errorf(
				"xz block header: uncompressed size: %s", err)
		}
	}
	for i := 0; i < info.flags.filters(); i++ {
		// TODO
	}
	panic("TODO")
}
