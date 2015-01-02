# xz

Native Go Language implementation of the xz LZMA2 compression.

This Go package is currently under development. You cannot use it even at your
own risk.

The classic LZMA decoder works now for the example files.

## TODO

1. Implement opReader based on opCodec.
2. Use opReader in lzma.Reader.
3. Change hash.Roller interface to elementar operations supported by
   functions.
4. Write the opWriter.
5. Write the opGenerator.
