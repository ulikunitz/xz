# TODO list

# Release v0.3

- Working LZMA2 implementation

# Release 1.0

1. Add godoc URL to README.md. (godoc.org)

# Package xz

- Provide user the capability to get uncompressed size before unpacking.

# Subpackage lzma2

## LZMA2 support

1. Create a package lzma2 that supports classic LZMA as well as LZMA2.

   a) Implement NewClassicReader using baseReader and test it.
   b) Implement baseWriter allowing the reuse of writerDict and opCodec.
   c) Implement NewClassicWriter based on baseWriter and test it.
   d) Implement NewReader and NewWriter using basedReader and baseWriter
      supporting LZMA2.

2. Remove lzma package.

## Optimizations

- Use radix trees (crit-bit trees) instead of the hash.

# lzmago binary

1. Put the functions in the xz/pack package to prevent reinventing the
   wheel. Those commands can then be used for optimization.
2. Add -c  flag

# Log

## 2015-04-04

Implemented baseReader by adapting code form lzma.Reader.

## 2015-04-03

The opCodec has been copied yesterday to lzma2. opCodec has a high
number of dependencies on other files in lzma2. Therefore I had to copy
almost all files from lzma.

## 2015-03-31

Removed only a TODO item. 

However in Francesco Campoy's presentation "Go for Javaneros
(Javaïstes?)" is the the idea that using an embedded field E, all the
methods of E will be defined on T. If E is an interface T satisfies E.

https://talks.golang.org/2014/go4java.slide#51

I have never used this, but it seems to be a cool idea.

## 2015-03-30

Finished the type writerDict and wrote a simple test.

## 2015-03-25

I started to implement the writerDict.

## 2015-03-24

After thinking long about the LZMA2 code and several false starts, I
have now a plan to create a self-sufficient lzma2 package that supports
the classic LZMA format as well as LZMA2. The core idea is to support a
baseReader and baseWriter type that support the basic LZMA stream
without any headers. Both types must support the reuse of dictionaries
and the opCodec.

## 2015-01-10

1. Implemented simple lzmago tool
2. Tested tool against large 4.4G file
    - compression worked correctly; tested decompression with lzma
    - decompression hits a full buffer condition
3. Fixed a bug in the compressor and wrote a test for it
4. Executed full cycle for 4.4 GB file; performance can be improved ;-)

## 2015-01-11

- Release v0.2 because of the working LZMA encoder and decoder
