package xz

import (
	"errors"
	"hash/crc32"
	"io"
)

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
	k, err = mw.Write(p[:4])
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
	g := crc.Sum32()
	p := make([]byte, 4)
	k, err = io.ReadFull(br.(io.Reader), p)
	n += k
	if err != nil {
		return records, n, err
	}
	if uint32LE(p) != g {
		return records, n, errors.New("xz: wrong checsum for index")
	}

	return records, n, nil
}
