// Copyright 2014-2017 Ulrich Kunitz. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// +build plan9

package main

import (
	"os"
	"os/signal"
)

// signalHandler establishes the signal handler for SIGTERM(1) and
// handles it in its own go routine. The returned quit channel must be
// closed to terminate the signal handler go routine.
func signalHandler(w *writer) chan<- struct{} {
	quit := make(chan struct{})
	sigch := make(chan os.Signal, 1)
	//signal.Notify(sigch, os.Interrupt, syscall.SIGPIPE)
	go func() {
		select {
		case <-quit:
			signal.Stop(sigch)
			return
		case <-sigch:
			w.removeTmpFile()
			os.Exit(7)
		}
	}()
	return quit
}