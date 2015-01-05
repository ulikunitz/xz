# xz

Native Go Language implementation of the xz LZMA2 compression.

This Go package is currently under development. You cannot use it even at your
own risk.

The classic LZMA decoder works now for the example files.

## TODO

1. Write the writer dictionary extension.
2. Review hash table implementation.
3. Write an op generator consisting of the dictionary and the hashes.
