package lzbase

import (
	"io"
	"log"

	"github.com/uli-go/xz/xlog"
)

// debug stores a reference to a logger. The xlog.Quiet logger doesn't print
// any messages.
var debug xlog.Logger = xlog.Quiet

// debugOn uses the log.Logger type to write information on the given writer.
// If w is nil no output will be written.
func debugOn(w io.Writer) {
	if w == nil {
		debug = xlog.Quiet
		return
	}
	debug = log.New(w, "", 0)
}

// debugOff() switches the debugging output off.
func debugOff() { debug = xlog.Quiet }
