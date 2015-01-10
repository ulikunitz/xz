# TODO list

# Subpackage lzma

# Implement golzma

Use a selected sets of flags from gzip

## LZMA2 preparation

1. include readProperties into readHeader
2. include writeProperties into writeHeader
3. Create
    NewRawReader(p *Properties) (*Reader, error)
   Use it in NewReader and NewWriterP
4. Create
    NewRawWriter(p *Properties) (*Writer, error)
5. Implement for Reader and Writer
    Reset(flags int, p *Properties)
