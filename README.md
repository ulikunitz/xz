# Package xz

This Go language package will provide support for the xz compression
support. The complete package is written in Go and doesn't use any
existing C library.

The package is currently under development. You cannot use it even on
your own risk.

At this point the decoder for classic LZMA files works. This decoder
will be used by the LZMA2 format, that is contained by the xz format.
