# TODO list

# Release v0.3

- Working LZMA2 implementation

# Release 1.0

1. Add godoc URL to README.md. (godoc.org)

# Package xz

- Provide user the capability to get uncompressed size before unpacking.

# Subpackage lzma

## LZMA2 preparation

1. Create a package lzma2 that supports classic LZMA as well as LZMA2.

   a) Implement writerDict that combines writing into the dictionary and
      hashing.
   b) Reuse readerDict.
   b) opCodec should also be implemented in a way that it can be reused.
   c) Implement baseReader allowing the reuse of readerDict and opCodec.
   d) Implement NewClassicReader and NewClassicWriter based on baseReader
      and baseWriter and test it.
   e) Implement NewReader and NewWriter using basedReader and baseWriter
      supporting LZMA2.

2. Remove lzma package and lzlib package.

## Optimizations

- Use radix trees (crit-bit trees) instead of the hash.

# lzmago binary

1. Put the functions in the xz/pack package to prevent reinventing the
   wheel. Those commands can then be used for optimization.
2. Add -c  flag

# Log

## 2015-01-10

1. Implemented simple lzmago tool
2. Tested tool against large 4.4G file
    - compression worked correctly; tested decompression with lzma
    - decompression hits a full buffer condition
3. Fixed a bug in the compressor and wrote a test for it
4. Executed full cycle for 4.4 GB file; performance can be improved ;-)

## 2015-01-11

- Release v0.2 because of the working LZMA encoder and decoder
