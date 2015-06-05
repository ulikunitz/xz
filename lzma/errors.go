package lzma

import "fmt"

type lzmaError struct {
	Msg string
}

func (err lzmaError) Error() string {
	return "lzma: " + err.Msg
}

type rangeError struct {
	Name  string
	Value interface{}
}

func (err rangeError) Error() string {
	return fmt.Sprintf("lzma: %s value %v out of range",
		err.Name, err.Value)
}

type negError struct {
	Name  string
	Value interface{}
}

func (err negError) Error() string {
	return fmt.Sprintf("lzma: %s (current value %v) must not be negative", err.Name, err.Value)
}

type limitError struct {
	Name string
}

func (err limitError) Error() string {
	return fmt.Sprintf("lzma: %s limit exceeded", err.Name)
}

var (
	errNoMatch       = lzmaError{"no match found"}
	errEmptyBuf      = lzmaError{"empty buffer"}
	errOptype        = lzmaError{"unsupported operation type"}
	errClosedWriter  = lzmaError{"writer is closed"}
	errClosedReader  = lzmaError{"reader is closed"}
	errWriterClosed  = lzmaError{"writer is closed"}
	errEarlyClose    = lzmaError{"writer closed with bytes remaining"}
	eos              = lzmaError{"end of stream"}
	errDataAfterEOS  = lzmaError{"data after end of streazm"}
	errUnexpectedEOS = lzmaError{"unexpected eos"}
	errAgain         = lzmaError{"buffer exhausted; repeat"}
	errReadLimit     = limitError{"read"}
	errWriteLimit    = limitError{"write"}
	errInt64         = lzmaError{"int64 values not representable as int"}
	errInt64Overflow = lzmaError{"int64 overflow detected"}
	errSpace         = lzmaError{"out of buffer space"}
)
