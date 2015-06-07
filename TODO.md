# TODO list

# Release v0.3

1. Simple lzmago supporting all the features of lzma. Links to
    lzma, unlzma and lzmacat are supported.
2. Execute test suite on:
    - x86 (Linux)
    - amd64 (Linux, Windows)
4. Check all external packages for license terms
5. Include all foreign licenses in the lzmago tool. Use go generate for
   this.
6. Add Copyright to all source code files including the markdown files.
   Write a go tool to do it and publish it.
7. Improve documentation
8. Create Release Notes as markdown file.
9. Check that package is indeed go-gettable.

# Release 1.0

1. Full functioning xzgo
2. Support by xzgo tool. Links for the classic tools should work.
    - xz
    - xzcat
    - unxz
3. Provide a manual page
4. Create Release Notes
5. Add godoc URL to README.md (godoc.org)
6. Resolve all issues.

# Package xz

- Implement the package using the LZMA2 support provided by LZMA2.
- Provide user the capability to get uncompressed size before unpacking.

# Subpackage lzma2

## LZMA2 support

1. Develop the package lzma2 using lzma streams.

## Optimizations

- Use radix trees (crit-bit trees) instead of the hash.

# lzmago binary

1. It seems that the main functionality of a compression tool is
   independent of the actual compression used. So abstract from it and
   implement functionality only once.
1. Put the functions in the xz/pack package to prevent reinventing the
   wheel. Those commands can then be used for optimization.
2. Add -c  flag

# Log

## 2014-06-05

The overflow issue was interesting to research, however Henry S. Warren
Jr. Hacker's Delight book was very helpful as usual and had the issue
explained perfectly. Fefe's information on his website was based on the
C FAQ and quite bad, because it didn't address the issue of -MININT ==
MININT.

## 2015-06-04

It has been a productive day. I improved the interface of lzma.Reader
and lzma.Writer and fixed the error handling.

## 2015-06-01

By computing the bit length of the LZMA operations I was able to
improve the greedy algorithm implementation. By using an 8 MByte buffer
the compression rate was not as good as for xz but already better then
gzip default. 

Compression is currently slow, but this is something we will be able to
improve over time.

## 2015-05-26

Checked the license of ogier/pflag. The binary lzmago binary should
include the license terms for the pflag library.

I added the endorsement clause as used by Google for the Go sources the
LICENSE file.

## 2015-05-22

The package lzb contains now the basic implementation for creating or
reading LZMA byte streams. It allows the support for the implementation
of the DAG-shortest-path algorithm for the compression function.

## 2015-04-23 

Completed yesterday the lzbase classes. I'm a little bit concerned that
using the components may require too much code, but on the other hand
there is a lot of flexibility.

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
