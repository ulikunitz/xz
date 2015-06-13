// +build darwin dragonfly freebsd netbsd openbsd

package term

import "syscall"

const ioctlGetTermios = syscall.TIOCGETA
