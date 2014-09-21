# Potential optimizations

## Range codec

Use 64 bits to write or read more than one byte to the underlying reader
or writer.

## Bit operations

Compute ntz32 and nlz32 using a Reiser table. The single % operation may
be faster than the multiplication and shift used now.
