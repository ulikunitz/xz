# Potential optimizations

## Codecs

All the codecs are used internally, so init methods instead of new
functions can save a number of allocations.

## Range codec

Use 64 bits to write or read more than one byte to the underlying reader
or writer.
