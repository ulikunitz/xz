# TODO list

# Subpackage lzma

## Prepare Release 0.2

1. Update README.md
2. Pull dev from master
3. Create tag

### LZMA2 preparation

1. Implement for Reader and Writer
    Reset(flags int, p *Properties) error
2. Create
    NewRawReader(p *Properties) (*Reader, error);
   use it in NewReader
3. Create
    NewRawWriter(p *Properties) (*Writer, error);
   use it in NewWriterP

## lzmago

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
