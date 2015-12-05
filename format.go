package xz

import (
	"bytes"
	"errors"
	"hash/crc32"
	"io"
)

/*** Header ***/

// headerMagic stores the magic bytes for the header
var headerMagic = []byte{0xfd, '7', 'z', 'X', 'Z', 0x00}

// headerLen defines the length of the stream header.
const headerLen = 12

// Constants for the checksum methods supported by xz.
const (
	fCRC32  byte = 0x1
	fCRC64       = 0x4
	fSHA256      = 0xa
)

// errInvalidFlags indicates that flags are invalid.
var errInvalidFlags = errors.New("xz: invalid flags")

// verifyFlags returns the error errInvalidFlags if the value is
// invalid.
func verifyFlags(flags byte) error {
	switch flags {
	case fCRC32, fCRC64, fSHA256:
		return nil
	default:
		return errInvalidFlags
	}
}

// writeHeader writes the stream header into the provided writer.
func writeHeader(w io.Writer, flags byte) (n int, err error) {
	if err = verifyFlags(flags); err != nil {
		return 0, err
	}

	// header magic
	k, err := w.Write(headerMagic)
	n += k
	if err != nil {
		return n, err
	}

	crc := crc32.NewIEEE()
	mw := io.MultiWriter(w, crc)

	// stream flags
	k, err = mw.Write([]byte{0, flags})
	n += k
	if err != nil {
		return n, err
	}

	// crc32
	p := make([]byte, 4)
	putUint32LE(p, crc.Sum32())
	k, err = w.Write(p)
	n += k

	return n, err
}

// Errors returned by readHeader.
var (
	errPadding     = errors.New("xz: found padding")
	errHeaderMagic = errors.New("xz: invalid header magic bytes")
)

// readHeader reads the stream header and returns the flags value. The
// function supports padding checks by reading four bytes first to
// return errPadding if they are all zero. So repeatedly calling the
// function will find eventually the stream header after the padding.
func readHeader(r io.Reader) (flags byte, n int, err error) {
	p := make([]byte, headerLen)

	// check for padding
	n, err = io.ReadFull(r, p[:4])
	if err != nil {
		return 0, n, err
	}
	if p[0] == 0 {
		for _, c := range p[1:4] {
			if c != 0 {
				return 0, n, errHeaderMagic
			}
		}
		return 0, n, errPadding
	}
	if !bytes.Equal(p[:4], headerMagic[:4]) {
		return 0, n, errHeaderMagic
	}

	k, err := io.ReadFull(r, p[4:])
	n += k
	if err != nil {
		return 0, n, err
	}

	// magic header
	if !bytes.Equal(headerMagic[4:], p[4:6]) {
		return 0, n, errHeaderMagic
	}

	// stream flags
	if p[6] != 0 {
		return 0, n, errInvalidFlags
	}
	flags = p[7]
	if err = verifyFlags(flags); err != nil {
		return flags, n, err
	}

	// checksum
	crc := crc32.NewIEEE()
	crc.Write(p[6:8])
	if uint32LE(p[8:]) != crc.Sum32() {
		return flags, n, errors.New("xz: invalid checksum for header")
	}

	return flags, n, nil
}

/*** Footer ***/

// footerLen defines the length of the footer.
const footerLen = 12

// footerMagic contains the footer magic bytes.
var footerMagic = []byte{'Y', 'Z'}

// writeFooter writes the footer with the given index size and flags
// into the writer.
func writeFooter(w io.Writer, indexSize uint32, flags byte) (n int, err error) {
	if err = verifyFlags(flags); err != nil {
		return 0, err
	}

	p := make([]byte, footerLen)
	// backward size (index size)
	putUint32LE(p[4:], indexSize)
	// flags
	p[9] = flags
	// footer magic
	copy(p[10:], footerMagic)

	// CRC-32
	crc := crc32.NewIEEE()
	crc.Write(p[4:10])
	putUint32LE(p, crc.Sum32())

	return w.Write(p)
}

// readFooter reads the stream footer.
func readFooter(r io.Reader) (indexSize uint32, flags byte, n int, err error) {
	p := make([]byte, footerLen)

	if n, err = io.ReadFull(r, p); err != nil {
		return 0, 0, n, err
	}

	// magic bytes
	if !bytes.Equal(p[10:], footerMagic) {
		return 0, 0, n, errors.New("xz: footer magic invalid")
	}

	// CRC-32
	crc := crc32.NewIEEE()
	crc.Write(p[4:10])
	if uint32LE(p) != crc.Sum32() {
		return 0, 0, n, errors.New("checksum error")
	}

	// backward size (index size)
	indexSize = uint32LE(p[4:])

	// flags
	if p[8] != 0 {
		return 0, 0, n, errInvalidFlags
	}
	flags = p[9]
	if err = verifyFlags(flags); err != nil {
		return indexSize, flags, n, errInvalidFlags
	}

	return indexSize, flags, n, nil
}

/*** Index ***/

// record describes a block in the xz file index.
type record struct {
	unpaddedSize     int64
	uncompressedSize int64
}

// readFrom reads the record from the byte reader
func (rec *record) readFrom(r io.ByteReader) (n int, err error) {
	u, k, err := readUvarint(r)
	n += k
	if err != nil {
		return n, err
	}
	rec.unpaddedSize = int64(u)
	if rec.unpaddedSize < 0 {
		return n, errors.New("xz: unpadded size negative")
	}

	u, k, err = readUvarint(r)
	n += k
	if err != nil {
		return n, err
	}
	rec.uncompressedSize = int64(u)
	if rec.uncompressedSize < 0 {
		return n, errors.New("xz: uncompressed size negative")
	}

	return n, nil
}

// writeTo writes the record into the writer
func (rec *record) writeTo(w io.Writer) (n int, err error) {
	// maximum length of a uvarint is 10
	p := make([]byte, 20)
	n = putUvarint(p, uint64(rec.unpaddedSize))
	n += putUvarint(p[n:], uint64(rec.uncompressedSize))
	return w.Write(p[:n])
}

func writeIndex(w io.Writer, index []record) (n int, err error) {
	crc := crc32.NewIEEE()
	mw := io.MultiWriter(w, crc)

	// index indicator
	k, err := mw.Write([]byte{0})
	n += k
	if err != nil {
		return n, err
	}

	// number of records
	p := make([]byte, 10)
	k = putUvarint(p, uint64(len(index)))
	k, err = mw.Write(p[:k])
	n += k
	if err != nil {
		return n, err
	}

	// list of records
	for _, rec := range index {
		k, err = rec.writeTo(mw)
		n += k
		if err != nil {
			return n, err
		}
	}

	// index padding
	if k = n % 4; k > 0 {
		k, err = mw.Write(make([]byte, 4-k))
		n += k
		if err != nil {
			return n, err
		}
	}

	// crc32 checksum
	putUint32LE(p, crc.Sum32())
	k, err = w.Write(p[:4])
	n += k

	return n, err
}

type bReader struct {
	io.Reader
	p []byte
}

func (br *bReader) ReadByte() (c byte, err error) {
	n, err := br.Read(br.p)
	if n == 1 {
		return br.p[0], nil
	}
	if err == nil {
		return 0, errors.New("xz: no data")
	}
	return 0, err
}

func byteReader(r io.Reader) io.ByteReader {
	if br, ok := r.(io.ByteReader); ok {
		return br
	}
	return &bReader{r, make([]byte, 1)}
}

func readIndexBody(r io.Reader) (records []record, n int, err error) {
	crc := crc32.NewIEEE()

	// index indicator; no error expected for writer
	crc.Write([]byte{0})

	br := byteReader(io.TeeReader(r, crc))

	// number of records
	u, k, err := readUvarint(br)
	n += k
	if err != nil {
		return nil, n, err
	}
	recLen := int(u)
	if recLen < 0 || uint64(recLen) != u {
		return nil, n, errors.New("xz: record number overflow")
	}

	// list of records
	records = make([]record, recLen)
	for i := range records {
		k, err = records[i].readFrom(br)
		n += k
		if err != nil {
			return records[:i], n, err
		}
	}

	// index padding
	if k = (n + 1) % 4; k > 0 {
		k = 4 - k
		for i := 0; i < k; i++ {
			c, err := br.ReadByte()
			if err != nil {
				return records, n, err
			}
			n += 1
			if c != 0 {
				return records, n, errors.New(
					"xz: non-zero byte in index padding")
			}
		}
	}

	// crc32
	s := crc.Sum32()
	p := make([]byte, 4)
	k, err = io.ReadFull(br.(io.Reader), p)
	n += k
	if err != nil {
		return records, n, err
	}
	if uint32LE(p) != s {
		return records, n, errors.New("xz: wrong checsum for index")
	}

	return records, n, nil
}
