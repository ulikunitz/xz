# TODO list

# Release v0.3

1. Working LZMA2 implementation
2. Support by lzmago tool
3. Improve documentation

# Release 1.0

1. Add godoc URL to README.md. (godoc.org)
2. Resolve all issues.

# Package xz

- Provide user the capability to get uncompressed size before unpacking.

# Subpackage lzma2

## LZMA2 support

1. Redesign lzbase
    1. Test interoperation of Reader and Writer
    2. Implement LimitedReader with a fixed number of bytes
        - LimitedReader checks after reading the given bytes
          that rd has MaybeEOS() is true
    3. Implement ReaderCounter that simply counts the bytes written
    4. Implement WriterCounter that simply counts the bytes read

2. Create the package LZMA2 using lzbase
    1. work on the design

3. Create the package LZMA using lzbase

4. Minimize the interface of lzbase.
    1. Work partically on ReaderDict and WriterDict.
        Both types have a lot functions that might not be required out
        of lzbase.
   

## Optimizations

- Use radix trees (crit-bit trees) instead of the hash.

# lzmago binary

1. Put the functions in the xz/pack package to prevent reinventing the
   wheel. Those commands can then be used for optimization.
2. Add -c  flag

# Log

## 2015-04-22

Implemented Reader and Writer during the Bayern game against Porto. The
second half gave me enough time.

## 2015-04-21

While showering today morning I discovered that the design for OpEncoder
and OpDecoder doesn't work, because encoding/decoding might depend on
the current status of the dictionary. This is not exactly the right way
to start the day.

Therefore we need to keep the Reader and Writer design. This time around
we simplify it by ignoring size limits. These can be added by wrappers
around the Reader and Writer interfaces. The Parameters type isn't
needed anymore.

However I will implement a ReaderState and WriterState type to use
static typing to ensure the right State object is combined with the
right lzbase.Reader and lzbase.Writer.

As a start I have implemented ReaderState and WriterState to ensure
that the state for reading is only used by readers and WriterState only
used by Writers. 

## 2015-04-20

Today I implemented the OpDecoder and tested OpEncoder and OpDecoder.

## 2015-04-08

Came up with a new simplified design for lzbase. I implemented already
the type State that replaces OpCodec.

## 2015-04-06

The new lzma package is now fully usable and lzmago is using it now. The
old lzma package has been completely removed.

## 2015-04-05

Implemented lzma.Reader and tested it.

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
