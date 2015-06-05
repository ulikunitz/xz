package lzma

import "github.com/uli-go/xz/basics/i64"

func add(x, y int64) int64 {
	z, overflow := i64.Add(x, y)
	if overflow {
		panic(errInt64Overflow)
	}
	return z
}
