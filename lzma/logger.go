package lzma

import (
	"io"
	"log"

	"github.com/uli-go/xz/xlog"
)

// Debug stores a reference to a logger. It may contain nil for no output.
var Debug xlog.Logger

// DebugOn uses the log.Logger type to write information on the given writer.
// If w is nil no output will be written.
func DebugOn(w io.Writer) {
	if w == nil {
		Debug = nil
		return
	}
	Debug = log.New(w, "", 0)
}

// DebugOff() switches the debugging output off.
func DebugOff() { Debug = nil }
