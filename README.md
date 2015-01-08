# Package xz

This Go language package will enable reading and writing of
xz-compressed files and streams. The complete package will be written in
Go and doesn't use any existing C library.

The package is currently under development. *You cannot use it even on
your own risk.*

# Progress

Encoding and decoding of LZMA files/byte streams is working and so we can
compress and decompress files. The LZMA byte streams need now be
embedded in readers and writers for the LZMA2 and xz container formats.
