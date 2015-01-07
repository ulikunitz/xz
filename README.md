# Package xz

This Go language package will enable reading and writing of
xz-compressed files and streams. The complete package will be written in
Go and doesn't use any existing C library.

The package is currently under development. You cannot use it even on
your own risk.

At this point the decoder for classic LZMA files works. The xz file
format is wrapper of LZMA2 chunks that are either compressed or in LZMA
format.
