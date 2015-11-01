// Package lzma2 provides readers and writers for the LZMA2 format. The
// format adds the capabilities flushing, parallel compression and
// uncompressed segments to the LZMA algorithm.
//
// The Reader and Writer allows the reading and writing of LZMA2 chunk
// sequences. They can be used to parallel compress or decompress LZMA2
// streams.
//
// FileReader and FileWriter allow the encoding and decoding of LZMA2
// files sequentially. The first LZMA2 chunk is preceded by dictionary
// capacity byte and the files include the end-of-stream chunk.
package lzma2
