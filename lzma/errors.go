package lzma

// Error represents an LZMA-specific error. At this point in time it ensures
// that the method Error prefixes the message Msg with the string "lzma - ".
type Error struct {
	Msg string
}

// Errors returns the error message with the prefix "lzma - ".
func (e Error) Error() string {
	return "lzma - " + e.Msg
}

// newError creates a new lzma error with the given message.
func newError(msg string) error {
	return Error{msg}
}
