// Copyright 2015 Ulrich Kunitz. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package lzma

import "io"

// NewReader creates a new LZMA reader.
func NewReader(lzma io.Reader) (r *Reader, err error) {
	p, err := readHeader(lzma)
	if err != nil {
		return nil, err
	}
	p.normalizeReaderSizes()
	r, err = NewStreamReader(lzma, *p)
	return
}
