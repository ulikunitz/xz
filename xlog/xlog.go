/*
Package xlog provides a Logger interface and supporting functions
to support control over debug output.

The Go standard library supports a log package that provides an interface to
log messages. Unfortunately it doesn't support the enabling or disabling of
such output. Calling the function on a nil Logger pointer, results in a panic.
During the development of the lzma package I needed a way to
disable and enable debug output in a package. A possibility would be to
ioutil.Discard, but it would do all the formatting before used.

The Logger interface is simple and it is supported by the log.Logger type. The
package provides currently only Print, Printf, Println for the interface but
this should be sufficient for debugging. If the Logger interface is nil, the
function don't do anything.

The glog package, full path github.com/golang/glog, provides more functionality
but depends on flag.Parse() to be called. This is not what I need for simple
debugging of test functions.
*/
package xlog

import "fmt"

// This package requires types to support this interface. The log.Logger type
// supports this interface.
type Logger interface {
	Output(calldepth int, s string) error
}

// Print outputs the arguments using the logger. If the logger is nil nothing
// will be printed.
func Print(l Logger, v ...interface{}) {
	if l != nil {
		l.Output(2, fmt.Sprint(v...))
	}
}

// Printf prints the arguments using the format string. If the logger argument
// is nil nothing will be printed.
func Printf(l Logger, format string, v ...interface{}) {
	if l != nil {
		l.Output(2, fmt.Sprintf(format, v...))
	}
}

// Println prints the arguments and adds a newline. If the logger argument is
// nil nothing will be printed.
func Println(l Logger, v ...interface{}) {
	if l != nil {
		l.Output(2, fmt.Sprintln(v...))
	}
}
