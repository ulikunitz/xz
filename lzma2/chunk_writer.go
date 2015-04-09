package lzma2

import (
	"bytes"
	"fmt"
	"io"

	"github.com/uli-go/xz/lzbase"
)

type chunkWriter struct {
	w    io.Writer
	ctrl control
	bw   lzbase.Writer
	buf  bytes.Buffer
}

func newChunkWriter(w io.Writer, oc *lzbase.OpCodec, ctrl control) (*chunkWriter, error) {
	if w == nil {
		return nil, newError("newChunkWriter argument w is nil")
	}
	if !ctrl.packed() {
		return nil, newError(
			"chunkWriter doesn't create unpacked chunks explicitly")
	}
	cw := &chunkWriter{w: w, ctrl: ctrl}
	err := lzbase.InitWriter(&cw.bw, &cw.buf, oc, lzbase.Parameters{})
	if err != nil {
		return nil, err
	}
	if cw.bw.Dict.Cap() < maxCopyUnpackedSize {
		return nil, newError(fmt.Sprintf(
			"writer dictionary capacity must greater or equal"+
				" %d bytes",
			maxCopyUnpackedSize))
	}
	return cw, nil
}
