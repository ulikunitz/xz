// SPDX-FileCopyrightText: © 2014 Ulrich Kunitz
//
// SPDX-License-Identifier: BSD-3-Clause

package xz

import "hash"

type noneHash struct{}

func (h noneHash) Write(p []byte) (n int, err error) { return len(p), nil }

func (h noneHash) Sum(b []byte) []byte { return b }

func (h noneHash) Reset() {}

func (h noneHash) Size() int { return 0 }

func (h noneHash) BlockSize() int { return 0 }

func newNoneHash() hash.Hash {
	return &noneHash{}
}
