// Package lzma provides a reader and a writer for LZMA streams.
//
// The NewReader function reads the classic LZMA header. NewWriter and
// NewWriterParam generate the header. To read and write LZMA streams
// without the header use NewStreamReader and NewStreamWriter.
//
// The package is written completely in Go and doesn't rely on any external
// library.
package lzma
