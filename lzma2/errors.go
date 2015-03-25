package lzma2

// lerror represents an LZMA-specific error. It currently adds the prefix
// "lzma2: " to all errors created in the package.
type lerror struct {
	msg string
}

// Error returns the error message with the prefix "lzma2: ".
func (e lerror) Error() string {
	return "lzma2: " + e.msg
}

// newError creates a new lzma error with the given message.
func newError(msg string) error {
	return lerror{msg}
}
