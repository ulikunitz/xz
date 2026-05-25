// Copyright 2014-2025 Ulrich Kunitz. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package xio provides tools to handle I/O operations. It contains the
// [WriteCloserStack] type supporting combining multiple WriterClosers as single
// [io.WriteCloser].
package xio

import (
	"errors"
	"io"
)

// WriteCloserStack allows to support multiple WriteClosers to be handled as
// single WriteCloser.
type WriteCloserStack struct {
	Stack []io.WriteCloser
}

// NewWriteCloserStack creates a new WriteCloserStack. It will have an an empty
// stack.
func NewWriteCloserStack() *WriteCloserStack {
	return &WriteCloserStack{}
}

// Write writes data to the top WriteCloser in the stack. If the stack is empty
// Write will always succeed.
func (w *WriteCloserStack) Write(p []byte) (n int, err error) {
	k := len(w.Stack)
	if k == 0 {
		return len(p), nil
	}
	return w.Stack[k-1].Write(p)
}

// Close closes all writers on the stack and combines the errors. It will clear
// the stack.
func (w *WriteCloserStack) Close() error {
	var errs []error
	for k := len(w.Stack) - 1; k >= 0; k-- {
		err := w.Stack[k].Close()
		errs = append(errs, err)
	}
	w.Stack = nil
	return errors.Join(errs...)
}

// Push adds a new WriteCloser to the top of the stack. It panics if the
// WriteCloser is nil.
func (w *WriteCloserStack) Push(wc io.WriteCloser) {
	if wc == nil {
		panic("cannot push nil WriteCloser onto stack")
	}
	w.Stack = append(w.Stack, wc)
}
