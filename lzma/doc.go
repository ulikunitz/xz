// Copyright 2015 Ulrich Kunitz. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package lzma allows the decoding and encoding of LZMA streams.
//
// It provides a Writer hand a Reader for decoding and encoding of LZMA
// stream with headers.
//
// The types Encoder and Decoder are provided to allow the decoding and
// encoding of raw LZMA streams as required by the implementation of the
// LZMA2 format.
//
// The package is written completely in Go and doesn't rely on any external
// library.
package lzma
