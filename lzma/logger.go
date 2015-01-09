package lzma

import (
	"io"
	"log"

	"github.com/uli-go/xz/xlog"
)

// debug stores a reference to a logger. It may contain nil for no output.
var debug xlog.Logger

// debugOn uses the log.Logger type to write information on the given writer.
// If w is nil no output will be written.
func debugOn(w io.Writer) {
	if w == nil {
		debug = nil
		return
	}
	debug = log.New(w, "", 0)
}

// debugOff() switches the debugging output off.
func debugOff() { debug = nil }
