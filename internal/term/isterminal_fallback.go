// Copyright 2014-2026 Ulrich Kunitz. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:build !darwin && !dragonfly && !freebsd && (!linux || appengine) && !netbsd && !openbsd && !windows
// +build !darwin,!dragonfly,!freebsd,!linux,!netbsd,!openbsd,!windows !darwin,!dragonfly,!freebsd,appengine,!netbsd,!openbsd,!windows

package term

// IsTerminal returns false because we don't have support for this
// platform.
func IsTerminal(fd uintptr) bool {
	return false
}
