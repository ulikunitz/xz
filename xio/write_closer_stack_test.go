// Copyright 2014-2025 Ulrich Kunitz. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package xio_test

import (
	"io"
	"os"

	"github.com/ulikunitz/xz"
	"github.com/ulikunitz/xz/xio"
)

func ExampleWriteCloserStack() {
	wcStack := xio.NewWriteCloserStack()
	defer wcStack.Close()

	f, err := os.CreateTemp("", "example_write_closer_stack-*.xz")
	if err != nil {
		panic(err)
	}
	wcStack.Push(f)

	z, err := xz.NewWriter(f)
	if err != nil {
		panic(err)
	}
	wcStack.Push(z)

	_, err = io.WriteString(wcStack, "The fox jumps over the lazy dog.\n")
	if err != nil {
		panic(err)
	}

	err = wcStack.Close()
	if err != nil {
		panic(err)
	}

	// Output:
}
