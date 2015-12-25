// Copyright 2015 Ulrich Kunitz. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package lzma allows the decoding and encoding of LZMA streams. The
// Reader and Writer supports the decoding and encoding of LZMA streams
// with headers. The types Encoder and Decoder are provided to allow the
// decoding and encoding of raw LZMA streams without headers. The lzma2
// package uses these types.
//
// The package is written completely in Go and doesn't rely on any external
// library.
package lzma
