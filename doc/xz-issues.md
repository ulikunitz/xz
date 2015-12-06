# Issues in the XZ file format

During the development of the xz package for Go a number of issues with
the xz file format were observed. They are documented here to help a
later development of an improved format.

# xz file format

The file format should be structured in packets that have all encoded
their length in the header. The Index must be fully parsed to identify
its end.

A better approach would also have been to store the index size in the
index footer. This way a simple flag in the stream flags could have
indicated whether an index is present or not. The current format
requires the index always to be present. With the default block size an
xz file has only one block and the index would not have been required.

The padding should allow direct mapping of the CRC values into memory, but it
wastes bytes bearing no information. This is certainly not optimal for a
compression format.

It might also not be necessary to provide the filters including their
parameters to be provided for each block.

# LZMA2 

LZMA2 consists of a series of chunks with a header byte. The header byte
has a different format depending on whether it is an uncompressed or
compressed chunk. This has the consequence a complete reset of state,
properties and dictionary is not possible with an uncompressed chunk.
The encoder has to keep a state variable tracking a dictionary reset in
an uncompressed chunk to ensure that the flags are added in the first
compressed chunk to follow. This complicates the implementation of the
encoder.
