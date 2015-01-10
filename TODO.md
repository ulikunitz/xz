# TODO list

# Subpackage lzma

## golzma

1. Implement a golzma tool that support the classic gzip flags
2. Test it on the big pgn files

## LZMA2 preparation

1. include readProperties into readHeader
2. include writeProperties into writeHeader
3. Create
    NewRawReader(p *Properties) (*Reader, error);
   use it in NewReader
4. Create
    NewRawWriter(p *Properties) (*Writer, error);
   use it in NewWriterP
5. Implement for Reader and Writer
    Reset(flags int, p *Properties) error
