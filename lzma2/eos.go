package lzma2

import "io"

// WriteEOS writes a null byte indicating the end of the stream. The end
// of stream marker must always be present in an LZMA2 stream.
func WriteEOS(w io.Writer) error {
	var p [1]byte
	_, err := w.Write(p[:])
	return err
}
