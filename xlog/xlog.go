/*
Package xlog provides a Logger interface and provides an implememntation of
this interface that keeps quiet.

A possibility would be to ioutil.Discard, but this would imply the cost of
formating the messages.

The Logger interface provides all methods that makes sense for a debug logger.

The glog package, full path github.com/golang/glog, provides more functionality
but depends on flag.Parse() to be called. This is not what I need for simple
debugging of test functions.
*/
package xlog

import "log"

// We use this interface to handle a logger.
type Logger interface {
	Print(v ...interface{})
	Printf(format string, v ...interface{})
	Println(v ...interface{})
	SetFlags(flags int)
	SetPrefix(prefix string)
	Flags() int
	Prefix() string
}

// A logger that keeps quiet.
var Quiet = &xLogger{flags: log.LstdFlags}

// xLogger is a logger that does nothing.
type xLogger struct {
	flags  int
	prefix string
}

func (x *xLogger) Flags() int                             { return x.flags }
func (x *xLogger) Prefix() string                         { return x.prefix }
func (x *xLogger) Print(v ...interface{})                 {}
func (x *xLogger) Printf(format string, v ...interface{}) {}
func (x *xLogger) Println(v ...interface{})               {}
func (x *xLogger) SetFlags(flags int)                     { x.flags = flags }
func (x *xLogger) SetPrefix(prefix string)                { x.prefix = prefix }
