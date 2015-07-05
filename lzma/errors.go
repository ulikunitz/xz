// Copyright 2015 Ulrich Kunitz. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package lzma

import "fmt"

// lzmaError represents a general lzma error. The output of the Error
// function is prefixed by the string "lzma: ".
type lzmaError struct {
	Msg string
}

// Error returns the error message for lzmaEror prefixed by "lzma: ".
func (err lzmaError) Error() string {
	return "lzma: " + err.Msg
}

// rangeError describes a situation where a value falls outside of its
// range.
type rangeError struct {
	Name  string
	Value interface{}
}

// Errors returns the error string for rangeError.
func (err rangeError) Error() string {
	return fmt.Sprintf("lzma: %s value %v out of range",
		err.Name, err.Value)
}

// The type negError indicates an error for a value that must not become
// negative.
type negError struct {
	Name  string
	Value interface{}
}

// Error returns the error message for negError.
func (err negError) Error() string {
	return fmt.Sprintf("lzma: %s (current value %v) must not be negative", err.Name, err.Value)
}

// limitError represents a violation of a limit.
type limitError struct {
	Name string
}

// Error returns the error message for limitError.
func (err limitError) Error() string {
	return fmt.Sprintf("lzma: %s limit exceeded", err.Name)
}

// Errors used by the lzma code.
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
