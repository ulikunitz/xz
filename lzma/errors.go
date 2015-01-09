package lzma

// Error represents an LZMA-specific error. It allows the testing against LZMA
// errors.
//
//  if _, ok := err.(lzma.Error); ok {
//  	fmt.Println("lzma error %s", err)
//  }
type Error struct {
	Msg string
}

// Error returns the error message with the prefix "lzma - ".
func (e Error) Error() string {
	return "lzma - " + e.Msg
}

// newError creates a new lzma error with the given message.
func newError(msg string) error {
	return Error{msg}
}
