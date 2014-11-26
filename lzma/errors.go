package lzma

// Error marks an internal lzma error.
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
