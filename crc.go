package xz

import (
	"hash"
	"hash/crc32"
	"hash/crc64"
)

type crc32Hash struct {
	hash.Hash32
	p []byte
}

func (h *crc32Hash) Sum(b []byte) []byte {
	putUint32LE(h.p[:], h.Hash32.Sum32())
	b = append(b, h.p...)
	return b
}

func newCRC32() hash.Hash {
	return &crc32Hash{Hash32: crc32.NewIEEE(), p: make([]byte, 4)}
}

type crc64Hash struct {
	hash.Hash64
	p []byte
}

func (h *crc64Hash) Sum(b []byte) []byte {
	putUint64LE(h.p, h.Hash64.Sum64())
	b = append(b, h.p...)
	return b
}

func newCRC64() hash.Hash {
	return &crc64Hash{Hash64: crc64.New(crc64Table), p: make([]byte, 8)}
}
