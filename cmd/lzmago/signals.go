//+build !windows

package main

import (
	"os"
	"syscall"
)

// termsigs contains a list of signals indicating termination of the
// program that will be handled by processLZMA
var termsigs = []os.Signal{
	syscall.SIGHUP,
	syscall.SIGINT,
	syscall.SIGQUIT,
	syscall.SIGILL,
	syscall.SIGABRT,
	syscall.SIGFPE,
	syscall.SIGKILL,
	syscall.SIGPIPE,
	syscall.SIGTERM,
	syscall.SIGUSR1,
	syscall.SIGUSR2,
	syscall.SIGXCPU,
	syscall.SIGXFSZ,
}
